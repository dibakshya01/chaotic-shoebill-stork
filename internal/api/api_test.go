package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/findings"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/score"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/store"
)

func testExperiment() model.Experiment {
	rf := false
	return model.Experiment{
		Meta:        model.ExperimentMeta{Name: "gw-03", Version: 1, Title: "Auth under degradation"},
		Family:      model.FamilyGateway,
		Description: "fail-closed under session degradation",
		AttackIDs:   []string{"T1078"}, OwaspIDs: []string{"API2:2023"},
		Injection:    model.Injection{Detonator: "k8s", Mode: model.ModeInject, Reversible: true, Duration: model.Duration(45 * time.Second)},
		Hypothesis:   model.Hypothesis{SteadyState: []model.Probe{{Source: model.SourcePrometheus, Query: "up", Expect: model.Expectation{Type: model.ExpectPresent}}}},
		Assertions:   []model.Assertion{{Stage: model.Prevented, Source: model.SourceGatewayLog, Query: "x", Expect: model.Expectation{Type: model.ExpectAbsent}, SecurityRelevant: true, RequireUUID: &rf, Justification: "all traffic"}},
		BlastRadius:  model.BlastRadius{MaxPods: 1, MaxPctReplicas: 5, MaxDurationSeconds: 60, MaxTrafficPct: 1},
		Capabilities: []string{"gateway-injection"},
	}
}

func testStore() *store.Memory {
	st := store.New()
	st.PutService(score.ServiceSpec{Service: model.Service{ID: "checkout", Name: "checkout", Tier: 1, Team: "checkout"},
		Expected: []model.Family{model.FamilyGateway}})
	now := time.Now()
	st.AddRun(store.RunView{ID: "r-1", Experiment: "gw-03", Service: model.Service{ID: "checkout", Name: "checkout", Tier: 1},
		Family: model.FamilyGateway, Env: model.EnvStage, Verdict: model.FailOpen, Raw: 0, At: now,
		Ladder: model.Ladder{{Stage: model.Prevented, Outcome: model.StageFail, FailOpen: true, Detail: "served unauth"}}})
	st.UpsertFinding(findings.Finding{ID: "f-1", Verdict: model.FailOpen, Severity: findings.Critical,
		ServiceID: "checkout", ServiceName: "checkout", Tier: 1, Family: model.FamilyGateway,
		Title: "Auth failed open", Status: findings.Open, OpenedAt: now})
	return st
}

func TestAPIRoutes(t *testing.T) {
	srv := httptest.NewServer(New(testStore(), []model.Experiment{testExperiment()}).Handler())
	defer srv.Close()

	get := func(path string) (int, string) {
		r, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(b)
	}

	for _, p := range []string{"/api/v1/healthz", "/api/v1/version", "/api/v1/scores",
		"/api/v1/experiments", "/api/v1/runs", "/api/v1/findings", "/api/v1/findings/failopen", "/api/v1/gamedays", "/"} {
		if code, _ := get(p); code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, code)
		}
	}

	// scores returns the fail-open-capped Tier-1 service.
	code, body := get("/api/v1/scores")
	if code != 200 || !strings.Contains(body, "fail_open_open") {
		t.Fatalf("scores unexpected: %d %s", code, body)
	}

	// a prod-canary plan with insufficient approvals + missing capability is refused.
	planBody := `{"experiment":"gw-03","tier":1,"env":"prod-canary","ring":"canary",
	  "scope":{"podCount":1,"pctOfReplicas":5,"durationSeconds":90,"pctTraffic":1},
	  "approvals":["a"],"grantedCapabilities":["prod-injection"]}`
	r, err := http.Post(srv.URL+"/api/v1/runs:plan", "application/json", strings.NewReader(planBody))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var plan struct {
		Allowed bool `json:"allowed"`
		Gates   []struct {
			Gate string `json:"gate"`
			OK   bool   `json:"ok"`
		} `json:"gates"`
	}
	_ = json.NewDecoder(r.Body).Decode(&plan)
	if plan.Allowed {
		t.Fatal("plan should be refused")
	}
	if len(plan.Gates) != 10 {
		t.Fatalf("expected all 10 gates in the preview, got %d", len(plan.Gates))
	}
	failed := map[string]bool{}
	for _, g := range plan.Gates {
		if !g.OK {
			failed[g.Gate] = true
		}
	}
	for _, want := range []string{"BUDGET", "APPROVAL", "CAPABILITY"} {
		if !failed[want] {
			t.Errorf("expected gate %s to fail in the preview", want)
		}
	}

	// validate endpoint rejects an invalid experiment.
	bad := `{"metadata":{"name":""},"family":"nope"}`
	vr, err := http.Post(srv.URL+"/api/v1/experiments:validate", "application/json", strings.NewReader(bad))
	if err != nil {
		t.Fatal(err)
	}
	defer vr.Body.Close()
	var val struct {
		Valid  bool     `json:"valid"`
		Errors []string `json:"errors"`
	}
	_ = json.NewDecoder(vr.Body).Decode(&val)
	if val.Valid || len(val.Errors) == 0 {
		t.Fatal("expected validation to fail for an invalid experiment")
	}
}
