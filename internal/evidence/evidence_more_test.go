package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

func TestVerifyUnsignedReceiptFails(t *testing.T) {
	_, v := newTestSigner(t)
	r := sampleReceipt("x", model.Pass) // never signed
	if err := v.Verify(r); err == nil {
		t.Fatal("verifying an unsigned receipt must fail")
	}
}

func TestLoadPublicKeyErrors(t *testing.T) {
	dir := t.TempDir()
	// missing file
	if _, err := LoadPublicKey(filepath.Join(dir, "nope.pub")); err == nil {
		t.Fatal("missing key file must error")
	}
	// bad length
	bad := filepath.Join(dir, "bad.pub")
	if err := os.WriteFile(bad, []byte("aGVsbG8=\n"), 0o644); err != nil { // "hello" — not 32 bytes
		t.Fatal(err)
	}
	if _, err := LoadPublicKey(bad); err == nil {
		t.Fatal("a wrong-length public key must error")
	}
}

func TestVerifyLedgerMissingHeadFails(t *testing.T) {
	dir := t.TempDir()
	s, v := newTestSigner(t)
	l, _ := OpenLedger(dir, s)
	_ = l.Append(sampleReceipt("a", model.Pass))
	_ = l.Append(sampleReceipt("b", model.Pass))
	// Remove the signed head — the chain is intact but truncation can no longer be ruled out.
	if err := os.Remove(filepath.Join(dir, headFile)); err != nil {
		t.Fatal(err)
	}
	rep, err := VerifyLedger(dir, v)
	if err != nil {
		t.Fatal(err)
	}
	if rep.OK {
		t.Fatal("a ledger with no signed head must not verify OK")
	}
	if !strings.Contains(strings.Join(rep.Errors, " "), "no signed head") {
		t.Fatalf("expected a 'no signed head' error, got %v", rep.Errors)
	}
}

func TestVerifyLedgerTamperedHeadFails(t *testing.T) {
	dir := t.TempDir()
	s, v := newTestSigner(t)
	l, _ := OpenLedger(dir, s)
	_ = l.Append(sampleReceipt("a", model.Pass))
	// Repoint the signed head at a key the verifier does not trust.
	hp := filepath.Join(dir, headFile)
	b, _ := os.ReadFile(hp)
	tampered := strings.Replace(string(b), `"key_id": "test-key"`, `"key_id": "attacker"`, 1)
	if tampered == string(b) {
		t.Fatal("precondition: head key_id not found to tamper")
	}
	_ = os.WriteFile(hp, []byte(tampered), 0o600)
	rep, _ := VerifyLedger(dir, v)
	if rep.OK {
		t.Fatal("a head signed by an untrusted key must fail verification")
	}
}

func TestReadReceiptsMalformedFails(t *testing.T) {
	dir := t.TempDir()
	s, _ := newTestSigner(t)
	l, _ := OpenLedger(dir, s)
	_ = l.Append(sampleReceipt("a", model.Pass))
	// Append a junk line to the ledger file.
	f, _ := os.OpenFile(filepath.Join(dir, ledgerFile), os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = f.WriteString("{not json\n")
	_ = f.Close()
	if _, err := l.Receipts(); err == nil {
		t.Fatal("a malformed ledger line must surface a parse error")
	}
}

func TestLedgerChainFieldsAreSet(t *testing.T) {
	dir := t.TempDir()
	s, _ := newTestSigner(t)
	l, _ := OpenLedger(dir, s)
	_ = l.Append(sampleReceipt("a", model.Pass))
	_ = l.Append(sampleReceipt("b", model.Pass))
	rs, err := l.Receipts()
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("want 2 receipts, got %d", len(rs))
	}
	if rs[0].Seq != 0 || rs[1].Seq != 1 {
		t.Fatalf("seq should be 0,1; got %d,%d", rs[0].Seq, rs[1].Seq)
	}
	if rs[0].PrevHash != "" {
		t.Fatal("genesis receipt must have an empty prev_hash")
	}
	_, h0, _ := contentHash(rs[0])
	if rs[1].PrevHash != h0 {
		t.Fatal("receipt 1 must chain to receipt 0's content hash")
	}
}
