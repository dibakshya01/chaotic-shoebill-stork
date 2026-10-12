package model

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"
)

func testKeys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	return pub, priv
}

func validCap(t *testing.T, priv ed25519.PrivateKey, now time.Time) Capability {
	t.Helper()
	c := Capability{
		ID:        "cap-1",
		RunUUID:   "uuid-1",
		Target:    "svc:checkout/ns:checkout-stage",
		Scopes:    []string{"detonator:mock", "action:container-kill"},
		IssuedAt:  now.Add(-time.Second),
		ExpiresAt: now.Add(60 * time.Second),
	}
	if err := c.Sign("gov-key", priv); err != nil {
		t.Fatalf("sign: %v", err)
	}
	return c
}

func TestCapabilityAuthorizeOK(t *testing.T) {
	pub, priv := testKeys(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c := validCap(t, priv, now)
	if err := c.Authorize(pub, now, "svc:checkout/ns:checkout-stage", "action:container-kill"); err != nil {
		t.Fatalf("expected authorized, got %v", err)
	}
}

func TestCapabilityExpired(t *testing.T) {
	pub, priv := testKeys(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c := validCap(t, priv, now)
	later := now.Add(2 * time.Minute)
	if err := c.Authorize(pub, later, c.Target, "detonator:mock"); !errors.Is(err, ErrCapExpired) {
		t.Fatalf("expected ErrCapExpired, got %v", err)
	}
}

func TestCapabilityForged(t *testing.T) {
	pub, priv := testKeys(t)
	otherPub, _ := testKeys(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c := validCap(t, priv, now)
	// Verified against a DIFFERENT public key → forgery/wrong-signer.
	if err := c.Authorize(otherPub, now, c.Target, "detonator:mock"); !errors.Is(err, ErrCapBadSig) {
		t.Fatalf("expected ErrCapBadSig, got %v", err)
	}
	_ = pub
}

func TestCapabilityTamperedScope(t *testing.T) {
	pub, priv := testKeys(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c := validCap(t, priv, now)
	// Tamper after signing: widen scope. Signature must now fail.
	c.Scopes = append(c.Scopes, "*")
	if err := c.Authorize(pub, now, c.Target, "action:anything"); !errors.Is(err, ErrCapBadSig) {
		t.Fatalf("expected ErrCapBadSig after tamper, got %v", err)
	}
}

func TestCapabilityScopeDenied(t *testing.T) {
	pub, priv := testKeys(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c := validCap(t, priv, now)
	if err := c.Authorize(pub, now, c.Target, "action:delete-everything"); !errors.Is(err, ErrCapScope) {
		t.Fatalf("expected ErrCapScope, got %v", err)
	}
}

func TestCapabilityTargetMismatch(t *testing.T) {
	pub, priv := testKeys(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c := validCap(t, priv, now)
	if err := c.Authorize(pub, now, "svc:payments/ns:prod", "detonator:mock"); !errors.Is(err, ErrCapTarget) {
		t.Fatalf("expected ErrCapTarget, got %v", err)
	}
}

func TestCapabilityEmptyCallerTargetRefused(t *testing.T) {
	pub, priv := testKeys(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c := validCap(t, priv, now) // target-scoped
	// An empty caller target must be a mismatch, not a skip (the Round-1 bypass fix).
	if err := c.Authorize(pub, now, "", "detonator:mock"); !errors.Is(err, ErrCapTarget) {
		t.Fatalf("empty caller target must be refused for a target-scoped cap, got %v", err)
	}
}

func TestCapabilityWildcardScope(t *testing.T) {
	pub, priv := testKeys(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c := Capability{
		ID: "cap-w", RunUUID: "u", Target: "t",
		Scopes:    []string{"detonator:*"},
		IssuedAt:  now.Add(-time.Second),
		ExpiresAt: now.Add(time.Minute),
	}
	if err := c.Sign("gov-key", priv); err != nil {
		t.Fatal(err)
	}
	if err := c.Authorize(pub, now, "t", "detonator:http"); err != nil {
		t.Fatalf("wildcard should allow detonator:http, got %v", err)
	}
	if err := c.Authorize(pub, now, "t", "action:x"); !errors.Is(err, ErrCapScope) {
		t.Fatalf("wildcard detonator:* must not allow action:x, got %v", err)
	}
}
