package safety

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// DefaultGovernor evaluates all ten gates and, on a clean pass, mints the signed
// capability that is the only thing a detonator will act on.
type DefaultGovernor struct {
	providers Providers
	keyID     string
	key       ed25519.PrivateKey
	now       func() time.Time
	newID     func() string
}

// GovernorOption customizes a DefaultGovernor.
type GovernorOption func(*DefaultGovernor)

// WithClock injects the time source (for deterministic tests).
func WithClock(now func() time.Time) GovernorOption {
	return func(g *DefaultGovernor) { g.now = now }
}

// WithIDGen injects the id/uuid generator (for deterministic tests).
func WithIDGen(gen func() string) GovernorOption {
	return func(g *DefaultGovernor) { g.newID = gen }
}

// NewGovernor builds a governor. The key signs minted capabilities; detonators
// verify against the corresponding public key.
func NewGovernor(providers Providers, keyID string, key ed25519.PrivateKey, opts ...GovernorOption) *DefaultGovernor {
	g := &DefaultGovernor{
		providers: providers,
		keyID:     keyID,
		key:       key,
		now:       time.Now,
		newID:     randomID,
	}
	for _, o := range opts {
		o(g)
	}
	return g
}

// PublicKey returns the governor's capability-signing public key.
func (g *DefaultGovernor) PublicKey() ed25519.PublicKey {
	return g.key.Public().(ed25519.PublicKey)
}

// Evaluate runs every gate (none short-circuited), then mints a capability iff clean.
func (g *DefaultGovernor) Evaluate(ctx context.Context, req RunRequest) (Decision, error) {
	if req.Now.IsZero() {
		req.Now = g.now()
	}

	var refusals []Refusal
	refusals = append(refusals, gateKillSwitch(ctx, g.providers)...)
	refusals = append(refusals, gateSEV(ctx, req, g.providers)...)
	refusals = append(refusals, gateBlackout(ctx, req, g.providers)...)
	refusals = append(refusals, gatePromotion(ctx, req, g.providers)...)
	refusals = append(refusals, gateCapability(req)...)
	refusals = append(refusals, gateApproval(req)...)
	constraints, budgetRefusals := evalBudget(req)
	refusals = append(refusals, budgetRefusals...)
	refusals = append(refusals, gateFaultDomain(ctx, req, g.providers)...)
	refusals = append(refusals, gateConcurrency(ctx, req, g.providers)...)
	proof, steadyRefusals := gateSteadyState(ctx, req, g.providers)
	refusals = append(refusals, steadyRefusals...)

	dec := Decision{
		Allowed:        len(refusals) == 0,
		Refusals:       refusals,
		Constraints:    constraints,
		PolicyVersion:  PolicyVersion,
		Preflight:      proof,
		GatesEvaluated: len(Gates),
	}
	if !dec.Allowed {
		return dec, nil
	}

	cap, err := g.mintCapability(req)
	if err != nil {
		return Decision{}, fmt.Errorf("mint capability: %w", err)
	}
	dec.Capability = cap
	return dec, nil
}

// mintCapability creates and signs the per-run, scoped, time-boxed capability.
func (g *DefaultGovernor) mintCapability(req RunRequest) (*model.Capability, error) {
	now := req.Now
	dur := time.Duration(req.Experiment.Injection.Duration)
	if dur <= 0 {
		dur = 30 * time.Second
	}
	// TTL covers the run plus a 20% margin and a small floor (matches the agent's
	// hard-deadline in the dead-man's-switch).
	ttl := dur + dur/5 + 10*time.Second

	target := CanonicalTarget(req.Run.Target)
	scopes := []string{
		"detonator:" + req.Experiment.Injection.Detonator,
		"env:" + string(req.Run.Target.Env),
	}
	if action, ok := req.Experiment.Injection.Spec["action"].(string); ok && action != "" {
		scopes = append(scopes, "action:"+action)
	}

	cap := &model.Capability{
		ID:        "cap-" + g.newID(),
		RunUUID:   g.newID(),
		Target:    target,
		Scopes:    scopes,
		IssuedAt:  now,
		ExpiresAt: now.Add(ttl),
	}
	if err := cap.Sign(g.keyID, g.key); err != nil {
		return nil, err
	}
	return cap, nil
}

// CanonicalTarget renders the canonical target identity a capability is scoped to.
func CanonicalTarget(t model.Target) string {
	return fmt.Sprintf("svc:%s/ns:%s", t.Service.ID, t.Namespace)
}

// randomID returns a hex-ish random identifier.
func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
