package safety

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// MaxSuppressionTTL is the hard server-side cap on how long an alert can be
// suppressed. Suppression exists to mute the *expected noise* of a run; it must
// never become a tool for hiding a real problem.
const MaxSuppressionTTL = 30 * time.Minute

// SuppressionRequest asks to mute specific alert rules for the duration of a run.
type SuppressionRequest struct {
	RunID      string
	AlertRules []string
	TTL        time.Duration
}

// Suppression errors.
var (
	ErrSuppressWildcard  = errors.New("wildcard alert suppression is forbidden")
	ErrSuppressAssertion = errors.New("refusing to suppress an alert the experiment asserts on")
	ErrSuppressTTL       = errors.New("suppression TTL exceeds the hard maximum")
)

// ValidateSuppression enforces the invariants: no wildcards, TTL within the cap, and
// — the load-bearing one — the suppression set can NEVER intersect the alert rules the
// experiment asserts on. Suppressing an assertion would let a decayed control score a
// false PASS.
func ValidateSuppression(req SuppressionRequest, exp model.Experiment) error {
	if req.TTL > MaxSuppressionTTL {
		return fmt.Errorf("%w (%s > %s)", ErrSuppressTTL, req.TTL, MaxSuppressionTTL)
	}
	asserted := map[string]struct{}{}
	for _, r := range exp.AssertedAlertRules() {
		asserted[r] = struct{}{}
	}
	for _, r := range req.AlertRules {
		if strings.Contains(r, "*") {
			return fmt.Errorf("%w: %q", ErrSuppressWildcard, r)
		}
		if _, isAssertion := asserted[r]; isAssertion {
			return fmt.Errorf("%w: %q", ErrSuppressAssertion, r)
		}
	}
	return nil
}
