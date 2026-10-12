package evidence

import (
	"bufio"
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

func fixedClock() func() time.Time {
	return func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
}

func newTestSigner(t *testing.T) (*Signer, *Verifier) {
	t.Helper()
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	s := NewSigner("test-key", priv, WithClock(fixedClock()))
	v := NewVerifier(map[string]ed25519.PublicKey{"test-key": pub})
	return s, v
}

func sampleReceipt(id string, verdict model.Verdict) *Receipt {
	return &Receipt{
		ReceiptID: id,
		Run: ReceiptRun{
			ID: "r-" + id, UUID: "uuid-" + id,
			DefinitionID: "mesh-06", DefinitionVersion: 1,
			Env: model.EnvStage, Service: model.Service{ID: "checkout", Name: "checkout", Tier: 1},
		},
		Governance: Governance{PolicyVersion: "v1", SafetyDecision: SafetyDecision{Allowed: true, GatesEvaluated: 10}},
		Preflight:  Preflight{SteadyStateHeld: true},
		Ladder: model.Ladder{
			{Stage: model.Prevented, Outcome: model.StagePass, SLOms: 0},
		},
		Verdict: verdict,
	}
}

func TestSignAndVerify(t *testing.T) {
	s, v := newTestSigner(t)
	r := sampleReceipt("a", model.Pass)
	if err := s.Sign(r); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if r.Signature == nil || r.Signature.Alg != "Ed25519" {
		t.Fatalf("expected Ed25519 signature, got %+v", r.Signature)
	}
	if err := v.Verify(r); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestTamperDetected(t *testing.T) {
	s, v := newTestSigner(t)
	r := sampleReceipt("a", model.Pass)
	if err := s.Sign(r); err != nil {
		t.Fatal(err)
	}
	// Flip the verdict after signing — the classic "make a FAIL look like a PASS".
	r.Verdict = model.FailOpen
	if err := v.Verify(r); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("expected ErrBadSignature after tamper, got %v", err)
	}
}

func TestUntrustedKey(t *testing.T) {
	s, _ := newTestSigner(t)
	r := sampleReceipt("a", model.Pass)
	if err := s.Sign(r); err != nil {
		t.Fatal(err)
	}
	empty := NewVerifier(map[string]ed25519.PublicKey{})
	if err := empty.Verify(r); !errors.Is(err, ErrUntrustedKey) {
		t.Fatalf("expected ErrUntrustedKey, got %v", err)
	}
}

func TestLedgerAppendAndVerify(t *testing.T) {
	dir := t.TempDir()
	s, v := newTestSigner(t)
	l, err := OpenLedger(dir, s)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, id := range []string{"a", "b", "c"} {
		if err := l.Append(sampleReceipt(id, model.Pass)); err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
	}
	rep, err := VerifyLedger(dir, v)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !rep.OK {
		t.Fatalf("expected OK ledger, got errors: %v", rep.Errors)
	}
	if rep.Count != 3 || rep.HeadSeq != 2 {
		t.Fatalf("expected 3 receipts / head seq 2, got count=%d head=%d", rep.Count, rep.HeadSeq)
	}

	// Re-open and append a 4th — chain must continue across process restarts.
	l2, err := OpenLedger(dir, s)
	if err != nil {
		t.Fatal(err)
	}
	if err := l2.Append(sampleReceipt("d", model.FailOpen)); err != nil {
		t.Fatal(err)
	}
	rep, _ = VerifyLedger(dir, v)
	if !rep.OK || rep.Count != 4 {
		t.Fatalf("expected OK 4-receipt ledger after reopen, got ok=%v count=%d errs=%v", rep.OK, rep.Count, rep.Errors)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	for sc.Scan() {
		if len(sc.Text()) > 0 {
			lines = append(lines, sc.Text())
		}
	}
	return lines
}

func TestLedgerReorderDetected(t *testing.T) {
	dir := t.TempDir()
	s, v := newTestSigner(t)
	l, _ := OpenLedger(dir, s)
	for _, id := range []string{"a", "b", "c"} {
		_ = l.Append(sampleReceipt(id, model.Pass))
	}
	path := filepath.Join(dir, ledgerFile)
	lines := readLines(t, path)
	// Swap the first two receipts — breaks seq order and the prev_hash chain.
	lines[0], lines[1] = lines[1], lines[0]
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, _ := VerifyLedger(dir, v)
	if rep.OK {
		t.Fatalf("expected reorder to be detected, but ledger verified OK")
	}
}

func TestLedgerTailTruncationDetected(t *testing.T) {
	dir := t.TempDir()
	s, v := newTestSigner(t)
	l, _ := OpenLedger(dir, s)
	for _, id := range []string{"a", "b", "c"} {
		_ = l.Append(sampleReceipt(id, model.Pass))
	}
	path := filepath.Join(dir, ledgerFile)
	lines := readLines(t, path)
	// Drop the last receipt. The surviving chain is internally valid, so ONLY the
	// signed head catches this.
	lines = lines[:len(lines)-1]
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, _ := VerifyLedger(dir, v)
	if rep.OK {
		t.Fatalf("expected tail truncation to be detected via signed head, but ledger verified OK")
	}
	joined := strings.Join(rep.Errors, " ")
	if !strings.Contains(joined, "TAIL TRUNCATION") && !strings.Contains(joined, "head") {
		t.Fatalf("expected a head/truncation error, got: %v", rep.Errors)
	}
}

func TestLoadOrCreateKeyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ed25519.key")
	priv1, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	priv2, err := LoadOrCreateKey(path) // second call loads the same key
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !priv1.Equal(priv2) {
		t.Fatal("LoadOrCreateKey did not return a stable key across calls")
	}
	pub, err := LoadPublicKey(path + ".pub")
	if err != nil {
		t.Fatalf("load pub: %v", err)
	}
	if !pub.Equal(priv1.Public().(ed25519.PublicKey)) {
		t.Fatal("public key file does not match private key")
	}
}
