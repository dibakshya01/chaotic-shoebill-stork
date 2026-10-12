package demo

import (
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/score"
)

// demoServices are the fictional services in the demo posture board.
func demoServices() []score.ServiceSpec {
	return []score.ServiceSpec{
		{Service: model.Service{ID: "checkout", Name: "checkout", Tier: 1, Team: "checkout"},
			Expected: []model.Family{model.FamilyMesh, model.FamilyGateway, model.FamilyIdentity, model.FamilyData, model.FamilyDetection}},
		{Service: model.Service{ID: "payments", Name: "payments", Tier: 1, Team: "payments", OnPaymentsPath: true},
			Expected: []model.Family{model.FamilyMesh, model.FamilyGateway, model.FamilyIdentity, model.FamilyData, model.FamilyDetection}},
		{Service: model.Service{ID: "catalog", Name: "catalog", Tier: 2, Team: "catalog"},
			Expected: []model.Family{model.FamilyMesh, model.FamilyGateway, model.FamilyNetwork, model.FamilyDetection}},
		{Service: model.Service{ID: "search", Name: "search", Tier: 2, Team: "search"},
			Expected: []model.Family{model.FamilyMesh, model.FamilyGateway, model.FamilyNetwork}},
		{Service: model.Service{ID: "profile", Name: "profile", Tier: 2, Team: "profile"},
			Expected: []model.Family{model.FamilyMesh, model.FamilyGateway, model.FamilyIdentity, model.FamilyData}},
	}
}

// familyMeta carries display + standards metadata for the representative experiment.
type familyMeta struct {
	name, title               string
	attack, owasp, compliance string
}

var famMeta = map[model.Family]familyMeta{
	model.FamilyMesh:      {"mesh-06-sidecar-fail-open", "Mesh authz under sidecar failure", "T1562.001", "API5:2023", "ISO-A.8.5"},
	model.FamilyGateway:   {"gw-03-session-degradation", "Auth under session-service degradation", "T1078", "API2:2023", "PCI-DSS-8.2"},
	model.FamilyIdentity:  {"id-01-authz-fail-closed", "Authorization fails closed when unreachable", "T1078.004", "API5:2023", "ISO-A.8.2"},
	model.FamilyNetwork:   {"net-06-metadata-access", "Cloud metadata endpoint is unreachable", "T1552.005", "A01:2025", "NIST-SC-7"},
	model.FamilyData:      {"data-01-tokenization-bypass", "PII tokenization holds under vault degradation", "T1530", "LLM02:2025", "DPDP-S.8"},
	model.FamilyDetection: {"det-01-rule-regression", "Tier-1 detection rule still fires", "T1562", "A09:2025", "NIST-AU-6"},
}

// demoExperiments builds one representative inject-mode experiment per family.
func demoExperiments() map[model.Family]model.Experiment {
	out := map[model.Family]model.Experiment{}
	rf := false
	for fam, m := range famMeta {
		out[fam] = model.Experiment{
			Meta:            model.ExperimentMeta{Name: m.name, Version: 1, Title: m.title},
			Family:          fam,
			Description:     m.title + " — a safe, bounded, reversible condition verifying the control holds.",
			AttackIDs:       []string{m.attack},
			OwaspIDs:        []string{m.owasp},
			ControlFamilies: []string{m.compliance},
			Injection: model.Injection{
				Detonator: "mock", Mode: model.ModeInject,
				Spec:     map[string]any{"action": "inject-" + string(fam)},
				Duration: model.Duration(45 * time.Second), Reversible: true,
			},
			Hypothesis: model.Hypothesis{SteadyState: []model.Probe{
				{Source: model.SourceMock, Query: "up", Expect: model.Expectation{Type: model.ExpectPresent}},
			}},
			Assertions: []model.Assertion{
				{Stage: model.Prevented, Source: model.SourceMock, Query: "unauth_success",
					Expect: model.Expectation{Type: model.ExpectAbsent}, SecurityRelevant: true, RequireUUID: &rf},
				{Stage: model.Detected, Source: model.SourceMock, Query: "detected",
					Expect: model.Expectation{Type: model.ExpectPresent}, SLOms: 60000},
				{Stage: model.Alerted, Source: model.SourceMock, Query: "alerted",
					Expect: model.Expectation{Type: model.ExpectPresent}, SLOms: 300000},
			},
			BlastRadius: model.BlastRadius{MaxPods: 1, MaxPctReplicas: 20, MaxDurationSeconds: 60},
		}
	}
	return out
}

// planItem is one scheduled run in the demo.
type planItem struct {
	service string
	family  model.Family
	scn     scenario
}

// demoPlan is a realistic matrix: mostly green, with three fail-opens on Tier-1 paths,
// an undetected miss, a slow detection, and deliberately-untested cells.
func demoPlan() []planItem {
	return []planItem{
		// checkout (Tier-1): a critical fail-open on the gateway, rest healthy; detection untested.
		{"checkout", model.FamilyMesh, scPass},
		{"checkout", model.FamilyGateway, scFailOpen},
		{"checkout", model.FamilyIdentity, scPass},
		{"checkout", model.FamilyData, scPass},
		// payments (Tier-1): a critical fail-open on data (raw PII), rest healthy.
		{"payments", model.FamilyMesh, scPass},
		{"payments", model.FamilyGateway, scPass},
		{"payments", model.FamilyIdentity, scPass},
		{"payments", model.FamilyData, scFailOpen},
		{"payments", model.FamilyDetection, scPass},
		// catalog (Tier-2): detection slow, rest healthy; network untested.
		{"catalog", model.FamilyMesh, scPass},
		{"catalog", model.FamilyGateway, scPass},
		{"catalog", model.FamilyDetection, scSlow},
		// search (Tier-2): a mesh miss (undetected), rest healthy.
		{"search", model.FamilyMesh, scUndetected},
		{"search", model.FamilyGateway, scPass},
		// profile (Tier-2): a fail-open on identity; mesh healthy; data untested.
		{"profile", model.FamilyMesh, scPass},
		{"profile", model.FamilyIdentity, scFailOpen},
		{"profile", model.FamilyGateway, scPass},
	}
}

func serviceByID(svcs []score.ServiceSpec, id string) model.Service {
	for _, s := range svcs {
		if s.Service.ID == id {
			return s.Service
		}
	}
	return model.Service{ID: id, Name: id, Tier: 2}
}
