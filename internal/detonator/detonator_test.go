package detonator

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/sim"
)

const target = "svc:checkout/ns:checkout-stage"

var now = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }

func validCap(t *testing.T, priv ed25519.PrivateKey, kind string) model.Capability {
	t.Helper()
	c := model.Capability{
		ID: "cap", RunUUID: "run-uuid-123", Target: target,
		Scopes:    []string{"detonator:" + kind, "env:stage", "action:*"},
		IssuedAt:  now().Add(-time.Second),
		ExpiresAt: now().Add(time.Minute),
	}
	if err := c.Sign("gov", priv); err != nil {
		t.Fatal(err)
	}
	return c
}

// contractDetonators returns the detonators to run the contract against, with a factory
// for a working Spec for each.
func contractDetonators(t *testing.T, pub ed25519.PublicKey) []struct {
	d    Detonator
	spec Spec
} {
	env := sim.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	return []struct {
		d    Detonator
		spec Spec
	}{
		{NewMock(env, pub, now), Spec{Target: target, Action: "container-kill"}},
		{NewObserve(pub, now), Spec{Target: target}},
		// WithAllowPrivateEgress: the test server is loopback; the egress guard is
		// exercised separately in TestHTTPDetonatorEgressGuard.
		{NewHTTP(pub, now, WithAllowPrivateEgress()), Spec{Target: target, Params: map[string]any{"url": srv.URL}}},
	}
}

func TestDetonatorContract(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	_, otherPriv, _ := ed25519.GenerateKey(nil)
	ctx := context.Background()

	for _, tc := range contractDetonators(t, pub) {
		kind := tc.d.Kind()
		t.Run(kind, func(t *testing.T) {
			// 1. No capability (zero value) -> refuse.
			if _, err := tc.d.Detonate(ctx, tc.spec, model.Capability{}); err == nil {
				t.Errorf("%s: detonated with NO capability", kind)
			}
			// 2. Expired capability -> refuse.
			expired := validCap(t, priv, kind)
			expired.ExpiresAt = now().Add(-time.Hour)
			_ = expired.Sign("gov", priv)
			if _, err := tc.d.Detonate(ctx, tc.spec, expired); err == nil {
				t.Errorf("%s: detonated with an EXPIRED capability", kind)
			}
			// 3. Mis-scoped capability (scoped to a different detonator) -> refuse.
			mis := validCap(t, priv, "some-other-detonator")
			if _, err := tc.d.Detonate(ctx, tc.spec, mis); err == nil {
				t.Errorf("%s: detonated with a MIS-SCOPED capability", kind)
			}
			// 4. Forged capability (signed by a different key) -> refuse.
			forged := validCap(t, otherPriv, kind)
			if _, err := tc.d.Detonate(ctx, tc.spec, forged); err == nil {
				t.Errorf("%s: detonated with a FORGED capability", kind)
			}
			// 5. Wrong-target capability -> refuse.
			wrongTarget := validCap(t, priv, kind)
			wrongTarget.Target = "svc:payments/ns:prod"
			_ = wrongTarget.Sign("gov", priv)
			if _, err := tc.d.Detonate(ctx, tc.spec, wrongTarget); err == nil {
				t.Errorf("%s: detonated with a WRONG-TARGET capability", kind)
			}

			// 6. Valid capability -> succeed, and weave the uuid into fingerprints.
			good := validCap(t, priv, kind)
			arts, err := tc.d.Detonate(ctx, tc.spec, good)
			if err != nil {
				t.Fatalf("%s: valid capability was refused: %v", kind, err)
			}
			if arts.UUID != good.RunUUID {
				t.Errorf("%s: artifacts uuid %q != cap uuid %q", kind, arts.UUID, good.RunUUID)
			}
			foundUUID := false
			for _, fp := range tc.d.Fingerprints(good.RunUUID) {
				if strings.Contains(fp.Value, good.RunUUID) {
					foundUUID = true
				}
			}
			if !foundUUID {
				t.Errorf("%s: fingerprints do not carry the run uuid", kind)
			}
			// Revert is idempotent and independently verifiable.
			if err := tc.d.Revert(ctx, tc.spec, good); err != nil {
				t.Errorf("%s: revert failed: %v", kind, err)
			}
			if ok, _ := tc.d.VerifyReverted(ctx, tc.spec); !ok {
				t.Errorf("%s: VerifyReverted returned false after revert", kind)
			}
		})
	}
}

func TestActionScopeEnforced(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	env := sim.New()
	d := NewMock(env, pub, now)
	// Capability authorizes the mock detonator but only the "container-kill" action.
	c := model.Capability{
		ID: "cap", RunUUID: "u", Target: target,
		Scopes:   []string{"detonator:mock", "action:container-kill"},
		IssuedAt: now().Add(-time.Second), ExpiresAt: now().Add(time.Minute),
	}
	if err := c.Sign("gov", priv); err != nil {
		t.Fatal(err)
	}
	// A spec with a DIFFERENT action must be refused — the action scope is load-bearing.
	if _, err := d.Detonate(context.Background(), Spec{Target: target, Action: "delete-everything"}, c); err == nil {
		t.Fatal("expected refusal: capability does not authorize action 'delete-everything'")
	}
	if env.Active() {
		t.Fatal("must not inject when the action is out of scope")
	}
	// The authorized action succeeds.
	if _, err := d.Detonate(context.Background(), Spec{Target: target, Action: "container-kill"}, c); err != nil {
		t.Fatalf("authorized action should succeed: %v", err)
	}
}

func TestHTTPDetonatorEgressGuard(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	cap := validCap(t, priv, "http")

	// Guarded (default) http detonator must refuse egress to the loopback test server.
	guarded := NewHTTP(pub, now)
	if _, err := guarded.Detonate(context.Background(), Spec{Target: target, Params: map[string]any{"url": srv.URL}}, cap); err == nil {
		t.Fatal("egress guard must block the loopback address")
	}
	// A non-http scheme is refused up front.
	if _, err := guarded.Detonate(context.Background(), Spec{Target: target, Params: map[string]any{"url": "ftp://example.com"}}, cap); err == nil {
		t.Fatal("non-http scheme must be refused")
	}
	// Blocked literal IPs across the ranges the guard covers.
	for _, u := range []string{
		"http://169.254.169.254/latest/meta-data/", // link-local metadata
		"http://[::1]:80/",                         // IPv6 loopback
		"http://0.0.0.0:80/",                       // unspecified
		"http://10.0.0.5/",                         // RFC1918 private
		"http://100.64.0.1/",                       // RFC6598 CGNAT
	} {
		if _, err := guarded.Detonate(context.Background(), Spec{Target: target, Params: map[string]any{"url": u}}, cap); err == nil {
			t.Fatalf("egress guard must block %s", u)
		}
	}
}
