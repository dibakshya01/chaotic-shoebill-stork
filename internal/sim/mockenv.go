// Package sim provides an in-process mock environment shared by the mock detonator
// and the mock signal source, so the full detonate -> observe -> verify -> score loop
// runs offline and deterministically. It models one target's control: before
// injection the control is healthy (steady state holds); while a condition is injected
// it responds according to programmed, scenario-specific signals.
package sim

import (
	"sync"
	"time"
)

// Response is what the mock environment returns for a query.
type Response struct {
	Found     bool    // did the query return anything
	Value     float64 // numeric value (for threshold/value expectations)
	Count     int     // number of results (for count expectations)
	LatencyMs int64   // latency of the signal relative to injection
	Err       bool    // the signal source "errored" (drives INCONCLUSIVE)
}

// MockEnv is a tiny, thread-safe state machine standing in for a real target + its
// observability. Program steady-state and during-injection responses, then Inject.
type MockEnv struct {
	mu         sync.Mutex
	active     bool
	reverted   bool
	uuid       string
	injectedAt time.Time

	steady map[string]Response
	during map[string]Response
}

// New builds an empty mock environment.
func New() *MockEnv {
	return &MockEnv{steady: map[string]Response{}, during: map[string]Response{}}
}

// ProgramSteady sets the response returned for a query before injection.
func (m *MockEnv) ProgramSteady(query string, r Response) *MockEnv {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.steady[query] = r
	return m
}

// ProgramDuring sets the response returned for a query while a condition is injected.
func (m *MockEnv) ProgramDuring(query string, r Response) *MockEnv {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.during[query] = r
	return m
}

// Inject marks a condition active, recording the correlation UUID.
func (m *MockEnv) Inject(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = true
	m.reverted = false
	m.uuid = uuid
	m.injectedAt = time.Now()
}

// Revert clears the condition. Idempotent.
func (m *MockEnv) Revert() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = false
	m.reverted = true
}

// Reverted reports whether the condition has been reverted (independent check path).
func (m *MockEnv) Reverted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reverted || !m.active
}

// Active reports whether a condition is currently injected.
func (m *MockEnv) Active() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// UUID returns the currently-injected correlation UUID.
func (m *MockEnv) UUID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.uuid
}

// Lookup returns the response for a query given the current state. During injection it
// returns the programmed during-response; otherwise the steady response. Unknown
// queries return a benign empty response (Found=false, no error).
func (m *MockEnv) Lookup(query string) Response {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active {
		if r, ok := m.during[query]; ok {
			return r
		}
		return Response{}
	}
	if r, ok := m.steady[query]; ok {
		return r
	}
	return Response{}
}
