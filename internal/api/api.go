// Package api serves the REST API and the embedded dashboard. It binds to localhost
// by default and is unauthenticated — an operator console, not a multi-tenant control
// surface. Experiment execution is CLI/operator-first; the API is read + safety-preview
// + validate (see build-plan.md FR-18).
package api

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/buildinfo"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/store"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/ui"
)

// Server holds the API dependencies.
type Server struct {
	store      *store.Memory
	exps       map[string]model.Experiment
	expList    []model.Experiment
	previewGov *safety.DefaultGovernor
	now        func() time.Time
}

// New builds a Server over a store and the catalog experiments.
func New(st *store.Memory, exps []model.Experiment) *Server {
	m := map[string]model.Experiment{}
	for _, e := range exps {
		m[e.Meta.Name] = e
	}
	// A preview governor for the safety-preview screen. It is ephemeral and only used
	// to render gate decisions; it never authorizes a real detonation.
	_, priv, _ := ed25519.GenerateKey(nil)
	promo := &safety.MapPromotionHistory{}
	now := time.Now
	for _, e := range exps {
		// Seed lower-env passes so the promotion gate can pass in a preview.
		promo.Record(e.Meta.Name, model.EnvPlayground, now().Add(-24*time.Hour))
		promo.Record(e.Meta.Name, model.EnvStage, now().Add(-24*time.Hour))
		promo.Record(e.Meta.Name, model.EnvPreprod, now().Add(-24*time.Hour))
	}
	providers := safety.Providers{
		KillSwitch: safety.StaticKillSwitch{}, Incidents: safety.NoIncidents{},
		Blackout: safety.StaticBlackout{}, Promotions: promo,
		Concurrency: safety.FreeConcurrency{Max: 5}, FaultDomains: safety.NoFaultDomainClash{},
		SteadyState: previewProber{},
	}
	gov := safety.NewGovernor(providers, "preview", priv)
	return &Server{store: st, exps: m, expList: exps, previewGov: gov, now: now}
}

// previewProber reports steady state as held (the preview demonstrates the policy
// gates, not live signal probing).
type previewProber struct{}

func (previewProber) Probe(_ context.Context, _ []model.Probe, _ map[string]any) safety.SteadyStateProof {
	return safety.SteadyStateProof{Held: true}
}

// Handler builds the HTTP handler (API routes + embedded UI).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"version": buildinfo.Version, "commit": buildinfo.Commit, "date": buildinfo.Date,
		})
	})
	mux.HandleFunc("GET /api/v1/scores", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.store.Scorecard(s.now()))
	})
	mux.HandleFunc("GET /api/v1/experiments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.experimentSummaries())
	})
	mux.HandleFunc("GET /api/v1/experiments/{id}", func(w http.ResponseWriter, r *http.Request) {
		if e, ok := s.exps[r.PathValue("id")]; ok {
			writeJSON(w, http.StatusOK, e)
			return
		}
		writeJSON(w, http.StatusNotFound, apiError("experiment not found"))
	})
	mux.HandleFunc("GET /api/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.store.ListRuns())
	})
	mux.HandleFunc("GET /api/v1/runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if run, ok := s.store.GetRun(r.PathValue("id")); ok {
			writeJSON(w, http.StatusOK, run)
			return
		}
		writeJSON(w, http.StatusNotFound, apiError("run not found"))
	})
	mux.HandleFunc("GET /api/v1/findings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.store.ListFindings())
	})
	mux.HandleFunc("GET /api/v1/findings/failopen", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.store.ListFailOpen())
	})
	mux.HandleFunc("GET /api/v1/gamedays", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.gameday())
	})
	mux.HandleFunc("POST /api/v1/experiments:validate", s.handleValidate)
	mux.HandleFunc("POST /api/v1/runs:plan", s.handlePlan)

	// Embedded dashboard at the root.
	mux.Handle("/", ui.Handler())
	return withCommon(mux)
}

// GateView is a per-gate decision row for the safety-preview screen.
type GateView struct {
	Gate   safety.GateID `json:"gate"`
	OK     bool          `json:"ok"`
	Reason string        `json:"reason,omitempty"`
	Remedy string        `json:"remedy,omitempty"`
}

func (s *Server) experimentSummaries() []map[string]any {
	out := make([]map[string]any, 0, len(s.expList))
	for _, e := range s.expList {
		out = append(out, map[string]any{
			"name": e.Meta.Name, "title": e.Meta.Title, "family": e.Family,
			"familyTitle": e.Family.Title(), "mode": e.Injection.Mode,
			"blastRadiusClass": e.BlastRadiusClass, "description": e.Description,
			"attackIds": e.AttackIDs, "owaspIds": e.OwaspIDs, "controlFamilies": e.ControlFamilies,
		})
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func apiError(msg string) map[string]string { return map[string]string{"error": msg} }

// withCommon adds a few safe headers.
func withCommon(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		h.ServeHTTP(w, r)
	})
}
