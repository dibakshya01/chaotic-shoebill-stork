package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/catalog"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
)

// maxBodyBytes bounds request bodies so a large POST cannot exhaust memory.
const maxBodyBytes = 1 << 20 // 1 MiB

// handleValidate validates a composed experiment (the Experiment Builder's check).
func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var e model.Experiment
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError("invalid experiment JSON: "+err.Error()))
		return
	}
	errs := catalog.Validate(e)
	msgs := make([]string, 0, len(errs))
	for _, er := range errs {
		msgs = append(msgs, er.Error())
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": len(errs) == 0, "errors": msgs})
}

type planRequest struct {
	Experiment          string      `json:"experiment"`
	ServiceID           string      `json:"serviceId"`
	ServiceName         string      `json:"serviceName"`
	Tier                model.Tier  `json:"tier"`
	Team                string      `json:"team"`
	OnPaymentsPath      bool        `json:"onPaymentsPath"`
	Namespace           string      `json:"namespace"`
	Env                 model.Env   `json:"env"`
	Ring                model.Ring  `json:"ring"`
	FaultDomainID       string      `json:"faultDomainId"`
	Scope               model.Scope `json:"scope"`
	Approvals           []string    `json:"approvals"` // team names
	GrantedCapabilities []string    `json:"grantedCapabilities"`
}

// handlePlan runs the Safety Governor WITHOUT detonating (the safety-preview screen).
func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req planRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError("invalid plan JSON: "+err.Error()))
		return
	}
	exp, ok := s.exps[req.Experiment]
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError("experiment not found: "+req.Experiment))
		return
	}
	approvals := make([]model.Approval, 0, len(req.Approvals))
	for _, team := range req.Approvals {
		approvals = append(approvals, model.Approval{Role: "approver", Team: team, Approver: "preview", At: s.now()})
	}
	svcName := req.ServiceName
	if svcName == "" {
		svcName = req.ServiceID
	}
	run := model.Run{
		ExperimentRef: req.Experiment,
		Target: model.Target{
			Service:   model.Service{ID: req.ServiceID, Name: svcName, Tier: req.Tier, Team: req.Team, OnPaymentsPath: req.OnPaymentsPath},
			Namespace: req.Namespace, Env: req.Env, Ring: req.Ring, FaultDomainID: req.FaultDomainID,
		},
		Scope:     req.Scope,
		Approvals: approvals,
	}
	dec, err := s.previewGov.Evaluate(r.Context(), safety.RunRequest{
		Experiment: exp, Run: run, GrantedCapabilities: req.GrantedCapabilities, Now: s.now(),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError(err.Error()))
		return
	}
	// Build a per-gate view: every gate, with pass/fail (preview shows the whole envelope).
	byGate := map[safety.GateID]safety.Refusal{}
	for _, rf := range dec.Refusals {
		if _, seen := byGate[rf.Gate]; !seen {
			byGate[rf.Gate] = rf
		}
	}
	gates := make([]GateView, 0, len(safety.Gates))
	for _, g := range safety.Gates {
		if rf, bad := byGate[g]; bad {
			gates = append(gates, GateView{Gate: g, OK: false, Reason: rf.Reason, Remedy: rf.Remedy})
		} else {
			gates = append(gates, GateView{Gate: g, OK: true})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"allowed":       dec.Allowed,
		"gates":         gates,
		"constraints":   dec.Constraints,
		"policyVersion": dec.PolicyVersion,
		"experiment":    map[string]any{"name": exp.Meta.Name, "title": exp.Meta.Title, "family": exp.Family, "mode": exp.Injection.Mode},
	})
}

// gameDayStep is one row of the game-day timeline.
type gameDayStep struct {
	N          int           `json:"n"`
	Experiment string        `json:"experiment"`
	Service    string        `json:"service"`
	Family     string        `json:"family"`
	Verdict    model.Verdict `json:"verdict"`
	At         time.Time     `json:"at"`
}

// gameday builds a read-only game-day view from recent runs (v1 is read-only).
func (s *Server) gameday() map[string]any {
	runs := s.store.ListRuns()
	var steps []gameDayStep
	failOpen, aborts, slo := 0, 0, 0
	n := 0
	for i := len(runs) - 1; i >= 0 && n < 9; i-- {
		rv := runs[i]
		if rv.Refused {
			continue
		}
		n++
		steps = append(steps, gameDayStep{
			N: n, Experiment: rv.Experiment, Service: rv.Service.Name,
			Family: rv.Family.Title(), Verdict: rv.Verdict, At: rv.At,
		})
		switch rv.Verdict {
		case model.FailOpen:
			failOpen++
		case model.Aborted:
			aborts++
		case model.FailSlow:
			slo++
		}
	}
	sc := s.store.Scorecard(s.now())
	return map[string]any{
		"title":       "Pre-event Security Readiness",
		"steps":       steps,
		"readiness":   sc.Summary.Efficacy,
		"failOpen":    failOpen,
		"sloBreaches": slo,
		"aborts":      aborts,
	}
}
