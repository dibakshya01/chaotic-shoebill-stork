package detonators

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	det "github.com/dibakshya01/chaotic-shoebill-stork/internal/detonator"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const target = "svc:checkout/ns:checkout-stage"

func now() time.Time { return time.Unix(1_700_000_000, 0).UTC() }

func validCap(t *testing.T, priv ed25519.PrivateKey, scopes []string) model.Capability {
	t.Helper()
	c := model.Capability{
		ID: "cap", RunUUID: "9f2c1a8e-aaaa-bbbb", Target: target,
		Scopes: scopes, IssuedAt: now().Add(-time.Second), ExpiresAt: now().Add(time.Minute),
	}
	if err := c.Sign("gov", priv); err != nil {
		t.Fatal(err)
	}
	return c
}

func testSpec() det.Spec {
	return det.Spec{Target: target, Action: "isolate", Params: map[string]any{
		"namespace": "checkout-stage", "labels": map[string]any{"app": "checkout"},
	}}
}

func TestNetpolDetonateAndRevert(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	cs := fake.NewSimpleClientset()
	d := NewNetworkPolicyDenyIngress(cs, pub, now)
	ctx := context.Background()
	spec := testSpec()
	cap := validCap(t, priv, []string{"detonator:netpol", "action:isolate", "env:stage"})

	// No capability -> refuse, no policy created.
	if _, err := d.Detonate(ctx, spec, model.Capability{}); err == nil {
		t.Fatal("detonated with no capability")
	}

	// Valid capability -> creates a deny-ingress policy with the run uuid woven in.
	arts, err := d.Detonate(ctx, spec, cap)
	if err != nil {
		t.Fatalf("detonate: %v", err)
	}
	if arts.UUID != cap.RunUUID {
		t.Fatalf("artifacts uuid %q != cap uuid %q", arts.UUID, cap.RunUUID)
	}
	nps, _ := cs.NetworkingV1().NetworkPolicies("checkout-stage").List(ctx, metav1.ListOptions{})
	if len(nps.Items) != 1 {
		t.Fatalf("expected 1 NetworkPolicy, got %d", len(nps.Items))
	}
	np := nps.Items[0]
	if np.Labels[det.RunLabel] != cap.RunUUID {
		t.Fatalf("policy must carry the run uuid label, got %v", np.Labels)
	}
	if np.Spec.PodSelector.MatchLabels["app"] != "checkout" {
		t.Fatalf("policy should target app=checkout, got %v", np.Spec.PodSelector.MatchLabels)
	}
	if len(np.Spec.Ingress) != 0 {
		t.Fatal("a deny-all-ingress policy must have no ingress rules")
	}

	// Not reverted while the policy exists.
	if ok, _ := d.VerifyReverted(ctx, spec); ok {
		t.Fatal("VerifyReverted should be false while the policy exists")
	}

	// Revert deletes it; VerifyReverted (independent List) then confirms.
	if err := d.Revert(ctx, spec, cap); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if ok, _ := d.VerifyReverted(ctx, spec); !ok {
		t.Fatal("VerifyReverted should be true after revert")
	}
	// Revert is idempotent.
	if err := d.Revert(ctx, spec, cap); err != nil {
		t.Fatalf("revert must be idempotent, got %v", err)
	}
}

func TestNetpolRefusesOutOfScopeAction(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	d := NewNetworkPolicyDenyIngress(fake.NewSimpleClientset(), pub, now)
	// Capability authorizes the netpol detonator but a DIFFERENT action.
	cap := validCap(t, priv, []string{"detonator:netpol", "action:something-else"})
	if _, err := d.Detonate(context.Background(), testSpec(), cap); err == nil {
		t.Fatal("must refuse when the action is out of scope")
	}
}

func TestNetpolRefusesForgedCapability(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	_, otherPriv, _ := ed25519.GenerateKey(nil)
	d := NewNetworkPolicyDenyIngress(fake.NewSimpleClientset(), pub, now)
	forged := validCap(t, otherPriv, []string{"detonator:netpol", "action:isolate"})
	if _, err := d.Detonate(context.Background(), testSpec(), forged); err == nil {
		t.Fatal("must refuse a capability signed by an untrusted key")
	}
}
