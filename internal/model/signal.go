package model

// SignalSource identifies where a steady-state probe or an assertion looks for
// evidence. A source that errors or is unavailable yields INCONCLUSIVE — never PASS.
type SignalSource string

const (
	SourcePrometheus   SignalSource = "prometheus"   // PromQL against a Prometheus-compatible API
	SourceAlertmanager SignalSource = "alertmanager" // alert state by rule name
	SourceHTTP         SignalSource = "http"         // an HTTP probe (status/body expectation)
	SourceK8sAudit     SignalSource = "k8saudit"     // Kubernetes audit events
	SourceAppLog       SignalSource = "applog"       // application logs
	SourceGatewayLog   SignalSource = "gwlog"        // gateway/proxy access logs
	SourceCanary       SignalSource = "canary"       // canary-token sweep
	SourceMock         SignalSource = "mock"         // in-process deterministic source (tests/demo)
)

// ExpectationType is the kind of comparison an assertion makes against a source.
type ExpectationType string

const (
	// ExpectPresent: the queried signal exists / returned a result. (e.g. an alert fired)
	ExpectPresent ExpectationType = "present"
	// ExpectAbsent: the signal must NOT exist. A match on a security-relevant "absent"
	// assertion is the definition of fail-open (the control permitted something).
	ExpectAbsent ExpectationType = "absent"
	// ExpectThreshold: a numeric value compared with an operator (gt/gte/lt/lte/eq).
	ExpectThreshold ExpectationType = "threshold"
	// ExpectCount: the number of returned series/rows compared with an operator.
	ExpectCount ExpectationType = "count"
	// ExpectValue: an exact value equality (e.g. HTTP status == 401).
	ExpectValue ExpectationType = "value"
	// ExpectPaged: a page was delivered (and optionally acknowledged).
	ExpectPaged ExpectationType = "paged"
)

// Op is a comparison operator for threshold/count/value expectations.
type Op string

const (
	OpGT  Op = "gt"
	OpGTE Op = "gte"
	OpLT  Op = "lt"
	OpLTE Op = "lte"
	OpEQ  Op = "eq"
	OpNEQ Op = "neq"
)

// Compare evaluates `got op want`.
func (o Op) Compare(got, want float64) bool {
	switch o {
	case OpGT:
		return got > want
	case OpGTE:
		return got >= want
	case OpLT:
		return got < want
	case OpLTE:
		return got <= want
	case OpEQ:
		return got == want
	case OpNEQ:
		return got != want
	default:
		return false
	}
}

// Expectation describes what a probe or assertion requires of a signal.
type Expectation struct {
	Type  ExpectationType `json:"type" yaml:"type"`
	Op    Op              `json:"op,omitempty" yaml:"op,omitempty"`
	Value float64         `json:"value,omitempty" yaml:"value,omitempty"`
}

// Probe is a steady-state measurement: "how do we know the hypothesis holds right now?"
type Probe struct {
	Name   string       `json:"name,omitempty" yaml:"name,omitempty"`
	Source SignalSource `json:"source" yaml:"source"`
	Query  string       `json:"query" yaml:"query"`
	Expect Expectation  `json:"expect" yaml:"expect"`
}

// Assertion is one rung of the ladder the experiment checks after detonation.
type Assertion struct {
	Stage  LadderStage  `json:"stage" yaml:"stage"`
	Source SignalSource `json:"source" yaml:"source"`
	Query  string       `json:"query" yaml:"query"`
	Expect Expectation  `json:"expect" yaml:"expect"`
	// SLOms is the maximum acceptable latency for this stage, in milliseconds.
	SLOms int64 `json:"slo_ms" yaml:"slo_ms"`
	// RequireUUID defaults to true. When false, Justification MUST be non-empty:
	// an assertion that is not UUID-scoped is a coincidence detector unless justified.
	RequireUUID   *bool  `json:"require_uuid,omitempty" yaml:"require_uuid,omitempty"`
	Justification string `json:"justification,omitempty" yaml:"justification,omitempty"`
	// SecurityRelevant marks an assertion whose "absent" expectation, if matched,
	// means the control permitted something it should not have (fail-open).
	SecurityRelevant bool `json:"security_relevant,omitempty" yaml:"security_relevant,omitempty"`
}

// RequiresUUID reports the effective value (default true).
func (a Assertion) RequiresUUID() bool {
	return a.RequireUUID == nil || *a.RequireUUID
}
