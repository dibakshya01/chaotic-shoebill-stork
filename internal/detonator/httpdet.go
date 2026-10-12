package detonator

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// HTTP detonates by sending a single, bounded HTTP request that embodies the
// adversarial condition (e.g. a request bearing a revoked session, a malformed JWT, or
// a canonical WAF probe) with the run uuid woven into the correlation header. It is
// stateless, so Revert is a no-op and VerifyReverted is always true.
//
// It is NOT wired to any shipped entrypoint (the demo registers only `mock`); it is
// provided for authorized use. To make it safe to wire later, egress is guarded by
// default: only http/https, no redirects, and the resolved address must not be a
// loopback/link-local/private/unspecified IP (SSRF defense, checked at dial time).
//
// Params (from the experiment injection spec): "method" (default GET), "url" (required).
type HTTP struct {
	pub          ed25519.PublicKey
	now          func() time.Time
	client       *http.Client
	allowPrivate bool
}

// HTTPOption customizes an HTTP detonator.
type HTTPOption func(*HTTP)

// WithAllowPrivateEgress disables the private/loopback egress guard. Intended for tests
// and for explicitly-authorized internal targets; never enable it by default.
func WithAllowPrivateEgress() HTTPOption {
	return func(h *HTTP) { h.allowPrivate = true }
}

// NewHTTP builds an HTTP detonator (egress-guarded by default).
func NewHTTP(governorPub ed25519.PublicKey, now func() time.Time, opts ...HTTPOption) *HTTP {
	if now == nil {
		now = time.Now
	}
	h := &HTTP{pub: governorPub, now: now}
	for _, o := range opts {
		o(h)
	}
	h.client = h.newClient()
	return h
}

func (h *HTTP) newClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("http detonator does not follow redirects")
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if !h.allowPrivate {
					if err := guardAddr(ctx, addr); err != nil {
						return nil, err
					}
				}
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}
}

// guardAddr resolves addr and rejects any private/loopback/link-local/unspecified IP,
// checked at dial time (mitigates DNS-rebinding).
func guardAddr(ctx context.Context, addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if isBlockedIP(ip.IP) {
			return fmt.Errorf("egress to %s (%s) blocked: loopback/link-local/private/unspecified", host, ip.IP)
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() || ip.IsUnspecified()
}

// Kind implements Detonator.
func (h *HTTP) Kind() string { return "http" }

func (h *HTTP) methodURL(spec Spec) (string, string) {
	method := http.MethodGet
	if m, ok := spec.Params["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}
	u, _ := spec.Params["url"].(string)
	return method, u
}

func validScheme(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("http detonator only allows http/https, got %q", u.Scheme)
	}
	return nil
}

// Plan implements Detonator.
func (h *HTTP) Plan(_ context.Context, spec Spec) (Plan, error) {
	method, u := h.methodURL(spec)
	if u == "" {
		return Plan{}, fmt.Errorf("http detonator requires a 'url' param")
	}
	if err := validScheme(u); err != nil {
		return Plan{}, err
	}
	return Plan{
		Detonator:  "http",
		Summary:    fmt.Sprintf("send %s %s with the run correlation header", method, u),
		Actions:    []string{fmt.Sprintf("%s %s", method, u), "set " + RunHeader},
		Reversible: true,
	}, nil
}

// Detonate verifies the capability and sends the request.
func (h *HTTP) Detonate(ctx context.Context, spec Spec, cap model.Capability) (Artifacts, error) {
	if err := cap.Authorize(h.pub, h.now(), spec.Target, "detonator:http"); err != nil {
		return Artifacts{}, fmt.Errorf("http detonator refused: %w", err)
	}
	if err := enforceActionScope(cap, spec); err != nil {
		return Artifacts{}, err
	}
	method, u := h.methodURL(spec)
	if u == "" {
		return Artifacts{}, fmt.Errorf("http detonator requires a 'url' param")
	}
	if err := validScheme(u); err != nil {
		return Artifacts{}, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return Artifacts{}, err
	}
	req.Header.Set(RunHeader, cap.RunUUID)
	resp, err := h.client.Do(req)
	if err != nil {
		return Artifacts{}, fmt.Errorf("http detonation %s %s: %w", method, u, err)
	}
	_ = resp.Body.Close()
	return Artifacts{
		UUID:         cap.RunUUID,
		Fingerprints: h.Fingerprints(cap.RunUUID),
		Notes:        []string{fmt.Sprintf("%s %s -> %d", method, u, resp.StatusCode)},
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
