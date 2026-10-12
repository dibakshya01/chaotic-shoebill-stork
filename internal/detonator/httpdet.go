package detonator

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// HTTP detonates by sending a single, bounded HTTP request that embodies the
// adversarial condition (e.g. a request bearing a revoked session, a malformed JWT, or
// a canonical WAF probe) with the run uuid woven into the correlation header. It is
// stateless, so Revert is a no-op and VerifyReverted is always true.
//
// Params (from the experiment injection spec): "method" (default GET), "url" (required).
type HTTP struct {
	pub    ed25519.PublicKey
	now    func() time.Time
	client *http.Client
}

// NewHTTP builds an HTTP detonator.
func NewHTTP(governorPub ed25519.PublicKey, now func() time.Time) *HTTP {
	if now == nil {
		now = time.Now
	}
	return &HTTP{pub: governorPub, now: now, client: &http.Client{Timeout: 10 * time.Second}}
}

// Kind implements Detonator.
func (h *HTTP) Kind() string { return "http" }

func (h *HTTP) methodURL(spec Spec) (string, string) {
	method := http.MethodGet
	if m, ok := spec.Params["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}
	url, _ := spec.Params["url"].(string)
	return method, url
}

// Plan implements Detonator.
func (h *HTTP) Plan(_ context.Context, spec Spec) (Plan, error) {
	method, url := h.methodURL(spec)
	if url == "" {
		return Plan{}, fmt.Errorf("http detonator requires a 'url' param")
	}
	return Plan{
		Detonator:  "http",
		Summary:    fmt.Sprintf("send %s %s with the run correlation header", method, url),
		Actions:    []string{fmt.Sprintf("%s %s", method, url), "set " + RunHeader},
		Reversible: true,
	}, nil
}

// Detonate verifies the capability and sends the request.
func (h *HTTP) Detonate(ctx context.Context, spec Spec, cap model.Capability) (Artifacts, error) {
	if err := cap.Authorize(h.pub, h.now(), spec.Target, "detonator:http"); err != nil {
		return Artifacts{}, fmt.Errorf("http detonator refused: %w", err)
	}
	method, url := h.methodURL(spec)
	if url == "" {
		return Artifacts{}, fmt.Errorf("http detonator requires a 'url' param")
	}
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return Artifacts{}, err
	}
	req.Header.Set(RunHeader, cap.RunUUID)
	resp, err := h.client.Do(req)
	if err != nil {
		return Artifacts{}, fmt.Errorf("http detonation %s %s: %w", method, url, err)
	}
	_ = resp.Body.Close()
	return Artifacts{
		UUID:         cap.RunUUID,
		Fingerprints: h.Fingerprints(cap.RunUUID),
		Notes:        []string{fmt.Sprintf("%s %s -> %d", method, url, resp.StatusCode)},
	}, nil
}

// Revert is a no-op (a single request leaves no state) but still enforces the cap.
func (h *HTTP) Revert(_ context.Context, spec Spec, cap model.Capability) error {
	return cap.Authorize(h.pub, h.now(), spec.Target, "detonator:http")
}

// VerifyReverted is always true for stateless probes.
func (h *HTTP) VerifyReverted(_ context.Context, _ Spec) (bool, error) { return true, nil }

// Fingerprints implements Detonator.
func (h *HTTP) Fingerprints(uuid string) []Fingerprint {
	return []Fingerprint{{Kind: "header", Where: RunHeader, Value: uuid}}
}
