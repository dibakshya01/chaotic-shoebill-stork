// Package store holds the platform's working state — experiments, the service
// registry, run results, and findings — behind a small in-memory implementation with
// JSON persistence (so `shoebill demo` can seed state that `shoebill serve` reads).
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/findings"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/score"
)

// RunView is a stored, completed (or refused) run, enough to list runs and to score.
type RunView struct {
	ID         string        `json:"id"`
	Experiment string        `json:"experiment"`
	Title      string        `json:"title,omitempty"`
	Service    model.Service `json:"service"`
	Family     model.Family  `json:"family"`
	Env        model.Env     `json:"env"`
	Ring       model.Ring    `json:"ring,omitempty"`
	Verdict    model.Verdict `json:"verdict,omitempty"`
	Raw        float64       `json:"raw"`
	Ladder     model.Ladder  `json:"ladder,omitempty"`
	At         time.Time     `json:"at"`
	ReceiptID  string        `json:"receipt_id,omitempty"`
	Refused    bool          `json:"refused"`
	Refusals   []string      `json:"refusals,omitempty"`
}

// snapshot is the JSON-serialized form of the store.
type snapshot struct {
	Experiments map[string]model.Experiment  `json:"experiments"`
	Services    map[string]score.ServiceSpec `json:"services"`
	Runs        []RunView                    `json:"runs"`
	Findings    map[string]findings.Finding  `json:"findings"`
}

// Memory is a thread-safe in-memory store.
type Memory struct {
	mu          sync.RWMutex
	experiments map[string]model.Experiment
	services    map[string]score.ServiceSpec
	runs        []RunView
	findings    map[string]findings.Finding
}

// New builds an empty store.
func New() *Memory {
	return &Memory{
		experiments: map[string]model.Experiment{},
		services:    map[string]score.ServiceSpec{},
		findings:    map[string]findings.Finding{},
	}
}

// --- experiments ---

func (m *Memory) PutExperiment(e model.Experiment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.experiments[e.Meta.Name] = e
}

func (m *Memory) GetExperiment(name string) (model.Experiment, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.experiments[name]
	return e, ok
}

func (m *Memory) ListExperiments() []model.Experiment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Experiment, 0, len(m.experiments))
	for _, e := range m.experiments {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Meta.Name < out[j].Meta.Name })
	return out
}

// --- services (registry) ---

func (m *Memory) PutService(s score.ServiceSpec) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services[s.Service.ID] = s
}

func (m *Memory) ListServices() []score.ServiceSpec {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]score.ServiceSpec, 0, len(m.services))
	for _, s := range m.services {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service.ID < out[j].Service.ID })
	return out
}

// --- runs ---

func (m *Memory) AddRun(r RunView) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs = append(m.runs, r)
}

func (m *Memory) ListRuns() []RunView {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := append([]RunView(nil), m.runs...)
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

func (m *Memory) GetRun(id string) (RunView, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.runs {
		if r.ID == id {
			return r, true
		}
	}
	return RunView{}, false
}

// --- findings ---

func (m *Memory) UpsertFinding(f findings.Finding) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.findings[f.ID] = f
}

func (m *Memory) ListFindings() []findings.Finding {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]findings.Finding, 0, len(m.findings))
	for _, f := range m.findings {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenedAt.After(out[j].OpenedAt) })
	return out
}

// ListFailOpen returns open fail-open findings, most recent first.
func (m *Memory) ListFailOpen() []findings.Finding {
	var out []findings.Finding
	for _, f := range m.ListFindings() {
		if f.IsOpenFailOpen() {
			out = append(out, f)
		}
	}
	return out
}

// --- scoring ---

// Scorecard builds the posture scorecard from the latest run per
// (service, family, experiment) and the open fail-open findings.
func (m *Memory) Scorecard(now time.Time) score.Scorecard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// latest run per (service, family, experiment)
	type key struct{ svc, exp string }
	latest := map[key]RunView{}
	for _, r := range m.runs {
		if r.Refused || r.Verdict == "" {
			continue
		}
		k := key{r.Service.ID, r.Experiment}
		if prev, ok := latest[k]; !ok || r.At.After(prev.At) {
			latest[k] = r
		}
	}
	var records []score.RunRecord
	for _, r := range latest {
		records = append(records, score.RunRecord{
			ServiceID: r.Service.ID, Family: r.Family, Experiment: r.Experiment,
			Verdict: r.Verdict, Raw: r.Raw, At: r.At,
		})
	}
	openFO := map[string]map[model.Family]bool{}
	for _, f := range m.findings {
		if f.IsOpenFailOpen() {
			if openFO[f.ServiceID] == nil {
				openFO[f.ServiceID] = map[model.Family]bool{}
			}
			openFO[f.ServiceID][f.Family] = true
		}
	}
	services := make([]score.ServiceSpec, 0, len(m.services))
	for _, s := range m.services {
		services = append(services, s)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Service.ID < services[j].Service.ID })
	return score.Build(services, records, openFO, now)
}

// --- persistence ---

// Save writes the store to a JSON file.
func (m *Memory) Save(path string) error {
	m.mu.RLock()
	snap := snapshot{Experiments: m.experiments, Services: m.services, Runs: m.runs, Findings: m.findings}
	m.mu.RUnlock()
	b, err := json.MarshalIndent(&snap, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Load reads the store from a JSON file. A missing file yields an empty store.
func Load(path string) (*Memory, error) {
	m := New()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	var snap snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, err
	}
	if snap.Experiments != nil {
		m.experiments = snap.Experiments
	}
	if snap.Services != nil {
		m.services = snap.Services
	}
	if snap.Findings != nil {
		m.findings = snap.Findings
	}
	m.runs = snap.Runs
	return m, nil
}
