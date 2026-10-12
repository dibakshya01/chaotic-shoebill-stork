package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/score"
)

func validExp() model.Experiment {
	f := false
	return model.Experiment{
		Meta:        model.ExperimentMeta{Name: "x", Version: 1},
		Family:      model.FamilyMesh,
		Description: "a valid experiment",
		AttackIDs:   []string{"T1078"},
		OwaspIDs:    []string{"API5:2023"},
		Injection:   model.Injection{Detonator: "mock", Mode: model.ModeInject, Reversible: true, Duration: model.Duration(45 * time.Second)},
		Hypothesis:  model.Hypothesis{SteadyState: []model.Probe{{Source: model.SourcePrometheus, Query: "up", Expect: model.Expectation{Type: model.ExpectPresent}}}},
		Assertions: []model.Assertion{{Stage: model.Prevented, Source: model.SourceGatewayLog, Query: "q",
			Expect: model.Expectation{Type: model.ExpectAbsent}, SecurityRelevant: true, RequireUUID: &f, Justification: "all traffic"}},
		BlastRadius: model.BlastRadius{MaxPods: 1, MaxDurationSeconds: 60},
	}
}

func hasErr(errs []error, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Error(), substr) {
			return true
		}
	}
	return false
}

func TestValidateGood(t *testing.T) {
	if errs := Validate(validExp()); len(errs) != 0 {
		t.Fatalf("valid experiment produced errors: %v", errs)
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	if len(Validate(model.Experiment{})) == 0 {
		t.Fatal("an empty experiment must produce errors")
	}
	// inject without reversible/duration/steadyState
	e := validExp()
	e.Injection.Reversible = false
	e.Injection.Duration = 0
	e.Hypothesis.SteadyState = nil
	e.BlastRadius.MaxDurationSeconds = 0
	errs := Validate(e)
	if !hasErr(errs, "reversible") || !hasErr(errs, "steady-state") || !hasErr(errs, "maxDurationSeconds") {
		t.Fatalf("expected reversible/steady-state/duration errors, got %v", errs)
	}
	// requireUuid:false with no justification
	e2 := validExp()
	e2.Assertions[0].Justification = ""
	if !hasErr(Validate(e2), "justification") {
		t.Fatal("requireUuid:false without a justification must error")
	}
	// expectedNoise intersecting an asserted alert rule
	e3 := validExp()
	e3.Assertions = append(e3.Assertions, model.Assertion{Stage: model.Alerted, Source: model.SourceAlertmanager,
		Query: "FooAlert", Expect: model.Expectation{Type: model.ExpectPresent}})
	e3.ExpectedNoise = []string{"FooAlert"}
	if !hasErr(Validate(e3), "suppression may never cover an assertion") {
		t.Fatalf("suppressing an asserted alert must error, got %v", Validate(e3))
	}
	// missing standards anchoring
	e4 := validExp()
	e4.AttackIDs = nil
	if !hasErr(Validate(e4), "ATT&CK") {
		t.Fatal("missing attackIds must error")
	}
}

func TestDuplicateNames(t *testing.T) {
	exps := []model.Experiment{
		{Meta: model.ExperimentMeta{Name: "a"}},
		{Meta: model.ExperimentMeta{Name: "a"}},
		{Meta: model.ExperimentMeta{Name: "b"}},
	}
	d := DuplicateNames(exps)
	if len(d) != 1 || d[0] != "a" {
		t.Fatalf("want [a], got %v", d)
	}
}

func TestValidateServices(t *testing.T) {
	good := []score.ServiceSpec{{Service: model.Service{ID: "s", Tier: 1}, Expected: []model.Family{model.FamilyMesh}}}
	if errs := ValidateServices(good); len(errs) != 0 {
		t.Fatalf("a tier-1 service should be valid: %v", errs)
	}
	bad := []score.ServiceSpec{
		{Service: model.Service{ID: "s", Tier: 5}},
		{Service: model.Service{ID: "", Tier: 1}},
		{Service: model.Service{ID: "t", Tier: 2}, Expected: []model.Family{"nope"}},
	}
	if len(ValidateServices(bad)) < 3 {
		t.Fatalf("expected tier/id/family errors, got %v", ValidateServices(bad))
	}
}

func TestLoadExperimentsAndServices(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "mesh"), 0o755)
	expYAML := `apiVersion: chaos.shoebill.dev/v1alpha1
kind: SecurityChaosExperiment
metadata: {name: test-exp, version: 1}
family: mesh
description: a loaded experiment
attackIds: [T1078]
owaspIds: [API5:2023]
injection: {detonator: observe, mode: observe_only}
assertions:
  - {stage: PREVENTED, source: prometheus, query: q, expect: {type: absent}}
`
	if err := os.WriteFile(filepath.Join(dir, "mesh", "e.yaml"), []byte(expYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	svcYAML := "services:\n  - {id: checkout, name: checkout, tier: 1, expected: [mesh]}\n"
	if err := os.WriteFile(filepath.Join(dir, "services.yaml"), []byte(svcYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	exps, err := LoadExperiments(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(exps) != 1 || exps[0].Meta.Name != "test-exp" {
		t.Fatalf("expected 1 experiment named test-exp, got %+v", exps)
	}
	if errs := Validate(exps[0]); len(errs) != 0 {
		t.Fatalf("loaded experiment should validate: %v", errs)
	}
	svcs, err := LoadServices(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 1 || svcs[0].Service.Tier != 1 {
		t.Fatalf("expected the checkout service, got %+v", svcs)
	}
}
