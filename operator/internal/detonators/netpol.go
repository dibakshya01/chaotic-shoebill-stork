// Package detonators holds the real in-cluster detonators (client-go). They implement
// the core detonator.Detonator interface and, like every detonator, verify the
// governor-minted capability before acting. Each action is bounded and reversible, and
// the revert is independently verified.
//
// NetworkPolicyDenyIngress isolates a target by applying a deny-all-ingress
// NetworkPolicy — a bounded, reversible way to ask "does the control fail closed when
// its dependency is unreachable?". Revert deletes the policy.
package detonators

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"strings"
	"time"

	det "github.com/dibakshya01/chaotic-shoebill-stork/internal/detonator"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const managedByLabel = "app.kubernetes.io/managed-by"
const managedByValue = "shoebill"

// NetworkPolicyDenyIngress is the `netpol` detonator.
type NetworkPolicyDenyIngress struct {
	client kubernetes.Interface
	pub    ed25519.PublicKey
	now    func() time.Time
}

// NewNetworkPolicyDenyIngress builds the detonator bound to a clientset and the
// governor's public key (used to verify every capability it is handed).
func NewNetworkPolicyDenyIngress(client kubernetes.Interface, governorPub ed25519.PublicKey, now func() time.Time) *NetworkPolicyDenyIngress {
	if now == nil {
		now = time.Now
	}
	return &NetworkPolicyDenyIngress{client: client, pub: governorPub, now: now}
}

// Kind implements Detonator.
func (d *NetworkPolicyDenyIngress) Kind() string { return "netpol" }

func (d *NetworkPolicyDenyIngress) namespace(spec det.Spec) string {
	if ns, ok := spec.Params["namespace"].(string); ok && ns != "" {
		return ns
	}
	// Fallback: parse "svc:<id>/ns:<namespace>" from the canonical target.
	if i := strings.Index(spec.Target, "/ns:"); i >= 0 {
		return spec.Target[i+4:]
	}
	return "default"
}

func (d *NetworkPolicyDenyIngress) selector(spec det.Spec) map[string]string {
	out := map[string]string{}
	if labels, ok := spec.Params["labels"].(map[string]any); ok {
		for k, v := range labels {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

func policyName(uuid string) string {
	short := uuid
	if len(short) > 8 {
		short = short[:8]
	}
	return "shoebill-deny-" + short
}

// Plan implements Detonator.
func (d *NetworkPolicyDenyIngress) Plan(_ context.Context, spec det.Spec) (det.Plan, error) {
	ns := d.namespace(spec)
	return det.Plan{
		Detonator:  "netpol",
		Summary:    fmt.Sprintf("apply a deny-all-ingress NetworkPolicy in namespace %q", ns),
		Actions:    []string{"create NetworkPolicy (deny ingress)", "label it with the run uuid"},
		Reversible: true,
	}, nil
}

// authorize checks the capability (signature/expiry/target/scope) and the action scope.
func (d *NetworkPolicyDenyIngress) authorize(spec det.Spec, cap model.Capability) error {
	if err := cap.Authorize(d.pub, d.now(), spec.Target, "detonator:netpol"); err != nil {
		return fmt.Errorf("netpol detonator refused: %w", err)
	}
	if spec.Action != "" && !cap.Allows("action:"+spec.Action) {
		return fmt.Errorf("netpol detonator refused: capability does not authorize action %q", spec.Action)
	}
	return nil
}

// Detonate verifies the capability, then applies the deny-ingress policy.
func (d *NetworkPolicyDenyIngress) Detonate(ctx context.Context, spec det.Spec, cap model.Capability) (det.Artifacts, error) {
	if err := d.authorize(spec, cap); err != nil {
		return det.Artifacts{}, err
	}
	ns := d.namespace(spec)
	np := &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName(cap.RunUUID),
			Namespace: ns,
			Labels:    map[string]string{det.RunLabel: cap.RunUUID, managedByLabel: managedByValue},
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: d.selector(spec)},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress},
			// No ingress rules = deny all ingress to the selected pods.
			Ingress: []netv1.NetworkPolicyIngressRule{},
		},
	}
	if _, err := d.client.NetworkingV1().NetworkPolicies(ns).Create(ctx, np, metav1.CreateOptions{}); err != nil {
		return det.Artifacts{}, fmt.Errorf("apply deny-ingress policy: %w", err)
	}
	return det.Artifacts{UUID: cap.RunUUID, Fingerprints: d.Fingerprints(cap.RunUUID)}, nil
}

// Revert deletes the policy. Idempotent (a missing policy is already reverted).
func (d *NetworkPolicyDenyIngress) Revert(ctx context.Context, spec det.Spec, cap model.Capability) error {
	if err := d.authorize(spec, cap); err != nil {
		return err
	}
	ns := d.namespace(spec)
	err := d.client.NetworkingV1().NetworkPolicies(ns).Delete(ctx, policyName(cap.RunUUID), metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// VerifyReverted checks — via an independent List — that no shoebill-managed policy
// remains in the namespace.
func (d *NetworkPolicyDenyIngress) VerifyReverted(ctx context.Context, spec det.Spec) (bool, error) {
	ns := d.namespace(spec)
	l, err := d.client.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{
		LabelSelector: managedByLabel + "=" + managedByValue,
	})
	if err != nil {
		return false, err
	}
	return len(l.Items) == 0, nil
}

// Fingerprints implements Detonator.
func (d *NetworkPolicyDenyIngress) Fingerprints(uuid string) []det.Fingerprint {
	return []det.Fingerprint{
		{Kind: "networkpolicy", Where: policyName(uuid), Value: uuid},
		{Kind: "label", Where: det.RunLabel, Value: uuid},
	}
}
