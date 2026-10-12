package safety

import (
	"context"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// The governor reads external state through small, injectable interfaces. The core
// ships deterministic in-memory implementations (used by the demo and tests); real
// adapters (incident webhook, service registry, etc.) implement the same interfaces.

// KillSwitch reports whether a global stop is engaged.
type KillSwitch interface {
	Engaged(ctx context.Context) bool
}

// IncidentProvider reports whether a blocking incident is open for a team.
type IncidentProvider interface {
	ActiveSev(ctx context.Context, team string) (active bool, id string)
}

// BlackoutCalendar reports whether now falls in a freeze window.
type BlackoutCalendar interface {
	InWindow(ctx context.Context, env model.Env, team string, now time.Time) (in bool, window string, nextEligible *time.Time)
}

// PromotionHistory reports the last time an experiment PASSed in an environment.
type PromotionHistory interface {
	LastPass(ctx context.Context, experiment string, env model.Env) (at time.Time, ok bool)
}

// ConcurrencyTracker reports whether a slot is free and the current usage.
type ConcurrencyTracker interface {
	Slot(ctx context.Context, req RunRequest) (free bool, used, max int)
}

// FaultDomainChecker reports whether a fault domain clashes with an in-flight run.
type FaultDomainChecker interface {
	Clash(ctx context.Context, faultDomainID string) (clash bool, withRun string)
}

// SteadyStateProber evaluates the experiment's steady-state probes right now.
type SteadyStateProber interface {
	Probe(ctx context.Context, probes []model.Probe, params map[string]any) SteadyStateProof
}

// Providers bundles the external-state dependencies of the governor.
type Providers struct {
	KillSwitch   KillSwitch
	Incidents    IncidentProvider
	Blackout     BlackoutCalendar
	Promotions   PromotionHistory
	Concurrency  ConcurrencyTracker
	FaultDomains FaultDomainChecker
	SteadyState  SteadyStateProber
}

// --- deterministic defaults (safe to run offline) --------------------------

// StaticKillSwitch is an in-memory kill switch.
type StaticKillSwitch struct{ On bool }

func (s StaticKillSwitch) Engaged(context.Context) bool { return s.On }

// NoIncidents reports no active incidents.
type NoIncidents struct{}

func (NoIncidents) ActiveSev(context.Context, string) (bool, string) { return false, "" }

// StaticIncidents returns a fixed active incident for the configured teams.
type StaticIncidents struct{ ActiveByTeam map[string]string }

func (s StaticIncidents) ActiveSev(_ context.Context, team string) (bool, string) {
	if id, ok := s.ActiveByTeam[team]; ok && id != "" {
		return true, id
	}
	return false, ""
}

// Window is a blackout freeze window.
type Window struct {
	Name  string
	Start time.Time
	End   time.Time
	Envs  []model.Env // empty = all environments
	Teams []string    // empty = all teams
}

// StaticBlackout is a calendar of fixed windows.
type StaticBlackout struct{ Windows []Window }

func (s StaticBlackout) InWindow(_ context.Context, env model.Env, team string, now time.Time) (bool, string, *time.Time) {
	var next *time.Time
	for i := range s.Windows {
		w := s.Windows[i]
		if !windowAppliesTo(w, env, team) {
			continue
		}
		if !now.Before(w.Start) && now.Before(w.End) {
			end := w.End
			return true, w.Name, &end
		}
		if now.Before(w.Start) {
			start := w.Start
			if next == nil || start.Before(*next) {
				next = &start
			}
		}
	}
	return false, "", next
}

func windowAppliesTo(w Window, env model.Env, team string) bool {
	if len(w.Envs) > 0 && !containsEnv(w.Envs, env) {
		return false
	}
	if len(w.Teams) > 0 && !containsStr(w.Teams, team) {
		return false
	}
	return true
}

// MapPromotionHistory is an in-memory promotion ledger keyed by experiment+env.
type MapPromotionHistory struct {
	Passes map[string]map[model.Env]time.Time
}

func (m MapPromotionHistory) LastPass(_ context.Context, experiment string, env model.Env) (time.Time, bool) {
	if byEnv, ok := m.Passes[experiment]; ok {
		if t, ok := byEnv[env]; ok {
			return t, true
		}
	}
	return time.Time{}, false
}

// Record notes a pass (used by the engine after a PASS verdict and by the demo seed).
func (m *MapPromotionHistory) Record(experiment string, env model.Env, at time.Time) {
	if m.Passes == nil {
		m.Passes = map[string]map[model.Env]time.Time{}
	}
	if m.Passes[experiment] == nil {
		m.Passes[experiment] = map[model.Env]time.Time{}
	}
	m.Passes[experiment][env] = at
}

// FreeConcurrency always grants a slot (single-run demo/tests).
type FreeConcurrency struct{ Max int }

func (f FreeConcurrency) Slot(context.Context, RunRequest) (bool, int, int) {
	max := f.Max
	if max == 0 {
		max = 5
	}
	return true, 0, max
}

// NoFaultDomainClash never clashes.
type NoFaultDomainClash struct{}

func (NoFaultDomainClash) Clash(context.Context, string) (bool, string) { return false, "" }

func containsEnv(s []model.Env, e model.Env) bool {
	for _, x := range s {
		if x == e {
			return true
		}
	}
	return false
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
