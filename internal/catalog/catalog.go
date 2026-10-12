// Package catalog loads and validates the experiment catalog (YAML files) and the
// service registry. The catalog is DATA, not code: a declarative hypothesis and its
// assertions, never a payload with side effects.
package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/score"
	"gopkg.in/yaml.v3"
)

// LoadExperiments reads every *.yaml / *.yml under dir (recursively) as an Experiment.
func LoadExperiments(dir string) ([]model.Experiment, error) {
	var exps []model.Experiment
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isYAML(path) || filepath.Base(path) == "services.yaml" {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		var e model.Experiment
		if uerr := yaml.Unmarshal(b, &e); uerr != nil {
			return fmt.Errorf("%s: %w", path, uerr)
		}
		exps = append(exps, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(exps, func(i, j int) bool { return exps[i].Meta.Name < exps[j].Meta.Name })
	return exps, nil
}

func isYAML(p string) bool {
	return strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")
}

// servicesFile is the registry schema.
type servicesFile struct {
	Services []serviceEntry `yaml:"services"`
}

type serviceEntry struct {
	ID             string         `yaml:"id"`
	Name           string         `yaml:"name"`
	Tier           model.Tier     `yaml:"tier"`
	Team           string         `yaml:"team"`
	OnPaymentsPath bool           `yaml:"onPaymentsPath"`
	Expected       []model.Family `yaml:"expected"`
}

// LoadServices reads the service registry (services.yaml) under dir.
func LoadServices(dir string) ([]score.ServiceSpec, error) {
	path := filepath.Join(dir, "services.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sf servicesFile
	if err := yaml.Unmarshal(b, &sf); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := make([]score.ServiceSpec, 0, len(sf.Services))
	for _, s := range sf.Services {
		out = append(out, score.ServiceSpec{
			Service:  model.Service{ID: s.ID, Name: s.Name, Tier: s.Tier, Team: s.Team, OnPaymentsPath: s.OnPaymentsPath},
			Expected: s.Expected,
		})
	}
	return out, nil
}

var validFamilies = func() map[model.Family]bool {
	m := map[model.Family]bool{}
	for _, f := range model.AllFamilies {
		m[f] = true
	}
	return m
}()

var validStages = map[model.LadderStage]bool{
	model.Prevented: true, model.Detected: true, model.Alerted: true, model.Paged: true, model.Ticketed: true,
}

var validSources = map[model.SignalSource]bool{
	model.SourcePrometheus: true, model.SourceAlertmanager: true, model.SourceHTTP: true,
	model.SourceK8sAudit: true, model.SourceAppLog: true, model.SourceGatewayLog: true,
	model.SourceCanary: true, model.SourceMock: true,
}

var validExpect = map[model.ExpectationType]bool{
	model.ExpectPresent: true, model.ExpectAbsent: true, model.ExpectThreshold: true,
	model.ExpectCount: true, model.ExpectValue: true, model.ExpectPaged: true,
}

// Validate checks an experiment against the catalog rules (FR-15). It returns every
// problem found, so an author sees the whole picture at once.
func Validate(e model.Experiment) []error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if strings.TrimSpace(e.Meta.Name) == "" {
		add("metadata.name is required")
	}
	if e.Meta.Version < 1 {
		add("metadata.version must be >= 1")
	}
	if !validFamilies[e.Family] {
		add("family %q is not one of the nine known families", e.Family)
	}
	if strings.TrimSpace(e.Description) == "" {
		add("description is required")
	}

	// Standards anchoring: at least one ATT&CK id and at least one OWASP/compliance id.
	if len(e.AttackIDs) == 0 {
		add("at least one ATT&CK technique id (attackIds) is required")
	}
	if len(e.OwaspIDs) == 0 && len(e.ControlFamilies) == 0 {
		add("at least one OWASP id (owaspIds) or compliance id (controlFamilies) is required")
	}

	// Injection.
	if strings.TrimSpace(e.Injection.Detonator) == "" {
		add("injection.detonator is required")
	}
	switch e.Injection.Mode {
	case model.ModeInject, model.ModeObserveOnly:
	default:
		add("injection.mode must be 'inject' or 'observe_only' (got %q)", e.Injection.Mode)
	}
	if e.Injection.Mode == model.ModeInject {
		if !e.Injection.Reversible {
			add("inject-mode experiments must declare reversible: true")
		}
		if e.Injection.Duration <= 0 {
			add("inject-mode experiments must declare a positive injection.duration")
		}
		if e.BlastRadius.MaxDurationSeconds <= 0 {
			add("inject-mode experiments must declare blastRadius.maxDurationSeconds > 0")
		}
		if len(e.Hypothesis.SteadyState) == 0 {
			add("inject-mode experiments must declare at least one steady-state probe")
		}
	}

	// Assertions.
	if len(e.Assertions) == 0 {
		add("at least one assertion is required")
	}
	for i, a := range e.Assertions {
		if !validStages[a.Stage] {
			add("assertion[%d]: invalid stage %q", i, a.Stage)
		}
		if !validSources[a.Source] {
			add("assertion[%d]: invalid source %q", i, a.Source)
		}
		if !validExpect[a.Expect.Type] {
			add("assertion[%d]: invalid expect.type %q", i, a.Expect.Type)
		}
		if a.RequireUUID != nil && !*a.RequireUUID && strings.TrimSpace(a.Justification) == "" {
			add("assertion[%d]: requireUuid:false demands a written justification", i)
		}
	}

	// Steady-state probes: valid source + expectation.
	for i, p := range e.Hypothesis.SteadyState {
		if !validSources[p.Source] {
			add("steadyState[%d]: invalid source %q", i, p.Source)
		}
		if !validExpect[p.Expect.Type] {
			add("steadyState[%d]: invalid expect.type %q", i, p.Expect.Type)
		}
	}

	// The load-bearing one: suppression can never cover an assertion.
	asserted := map[string]bool{}
	for _, r := range e.AssertedAlertRules() {
		asserted[r] = true
	}
	for _, n := range e.ExpectedNoise {
		if asserted[n] {
			add("expectedNoise %q is also an asserted alert rule — suppression may never cover an assertion", n)
		}
	}
	return errs
}

// DuplicateNames returns the experiment names that appear in more than one file (sorted).
// Duplicate names would silently shadow each other, so `catalog validate` rejects them.
func DuplicateNames(exps []model.Experiment) []string {
	count := map[string]int{}
	for _, e := range exps {
		count[e.Meta.Name]++
	}
	var dups []string
	for name, n := range count {
		if n > 1 {
			dups = append(dups, name)
		}
	}
	sort.Strings(dups)
	return dups
}

// ValidateServices checks the registry (every service has a tier in {1,2,3}).
func ValidateServices(specs []score.ServiceSpec) []error {
	var errs []error
	for _, s := range specs {
		if s.Service.ID == "" {
			errs = append(errs, fmt.Errorf("service with empty id"))
		}
		if s.Service.Tier < 1 || s.Service.Tier > 3 {
			errs = append(errs, fmt.Errorf("service %q: tier must be 1, 2, or 3 (got %d)", s.Service.ID, s.Service.Tier))
		}
		for _, f := range s.Expected {
			if !validFamilies[f] {
				errs = append(errs, fmt.Errorf("service %q: expected family %q is unknown", s.Service.ID, f))
			}
		}
	}
	return errs
}
