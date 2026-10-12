package model

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Capability is a signed, run-scoped, time-boxed authorization to detonate. It is
// minted ONLY by the Safety Governor on an Allowed decision (see internal/safety),
// and verified by each Detonator before it acts. This is what makes the governor
// the only door to injection *structurally* rather than by convention: a detonator
// handed no capability, an expired one, one scoped to a different target, or one not
// signed by the trusted governor key will refuse.
type Capability struct {
	ID        string    `json:"id"`
	RunUUID   string    `json:"run_uuid"`
	Target    string    `json:"target"` // canonical target identity, e.g. "svc:checkout/ns:checkout-stage"
	Scopes    []string  `json:"scopes"` // allowed actions, e.g. "detonator:mock", "action:container-kill"
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	KeyID     string    `json:"key_id"`
	Sig       string    `json:"sig,omitempty"` // base64 Ed25519 over signingBytes()
}

// Errors returned by capability verification.
var (
	ErrCapUnsigned = errors.New("capability is unsigned")
	ErrCapExpired  = errors.New("capability has expired")
	ErrCapNotYet   = errors.New("capability is not yet valid")
	ErrCapBadSig   = errors.New("capability signature does not verify (forged or altered)")
	ErrCapScope    = errors.New("capability does not authorize this action")
	ErrCapTarget   = errors.New("capability is scoped to a different target")
)

// signingBytes returns the deterministic bytes signed/verified (the capability sans Sig).
func (c Capability) signingBytes() ([]byte, error) {
	clone := c
	clone.Sig = ""
	return json.Marshal(&clone)
}

// Sign signs the capability with the governor's private key. Used only by the governor.
func (c *Capability) Sign(keyID string, priv ed25519.PrivateKey) error {
	c.KeyID = keyID
	b, err := c.signingBytes()
	if err != nil {
		return err
	}
	c.Sig = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, b))
	return nil
}

// Verify checks the signature against the trusted governor public key and the
// validity window. It does NOT check scope/target — use Authorize for a full check.
func (c Capability) Verify(pub ed25519.PublicKey, now time.Time) error {
	if c.Sig == "" {
		return ErrCapUnsigned
	}
	b, err := c.signingBytes()
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(c.Sig)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCapBadSig, err)
	}
	if !ed25519.Verify(pub, b, sig) {
		return ErrCapBadSig
	}
	if now.Before(c.IssuedAt) {
		return ErrCapNotYet
	}
	if !now.Before(c.ExpiresAt) {
		return ErrCapExpired
	}
	return nil
}

// Allows reports whether the capability's scopes authorize the given action. A scope
// of the form "prefix:*" matches any action with that prefix; "*" matches anything.
func (c Capability) Allows(action string) bool {
	for _, s := range c.Scopes {
		if s == "*" || s == action {
			return true
		}
		if strings.HasSuffix(s, ":*") && strings.HasPrefix(action, strings.TrimSuffix(s, "*")) {
			return true
		}
	}
	return false
}

// Authorize is the full check a detonator performs before acting: valid signature,
// unexpired, correct target, and the action within scope.
func (c Capability) Authorize(pub ed25519.PublicKey, now time.Time, target, action string) error {
	if err := c.Verify(pub, now); err != nil {
		return err
	}
	if c.Target != "" && target != "" && c.Target != target {
		return fmt.Errorf("%w: cap=%q action-target=%q", ErrCapTarget, c.Target, target)
	}
	if !c.Allows(action) {
		return fmt.Errorf("%w: %q not in %v", ErrCapScope, action, c.Scopes)
	}
	return nil
}
