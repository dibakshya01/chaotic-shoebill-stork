package demo

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/evidence"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/store"
)

func TestSeedProducesScorecardAndVerifiableLedger(t *testing.T) {
	base := t.TempDir()
	res, err := Seed(base)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if res.Runs == 0 {
		t.Fatal("expected some runs")
	}
	if res.FailOpen < 1 {
		t.Fatalf("expected at least one fail-open finding, got %d", res.FailOpen)
	}
	eff := res.Scorecard.Summary.Efficacy
	if eff <= 0 || eff >= 1 {
		t.Fatalf("efficacy should be in (0,1), got %v", eff)
	}
	if res.Scorecard.Summary.FailOpenOpen < 1 {
		t.Fatal("scorecard summary should report open fail-opens")
	}

	// The persisted state reloads and re-scores.
	st, err := store.Load(StatePath(base))
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if len(st.ListFailOpen()) < 1 {
		t.Fatal("reloaded store should contain fail-open findings")
	}
	sc := st.Scorecard(time.Now())
	if sc.Summary.CoverageTotal == 0 {
		t.Fatal("reloaded scorecard should have coverage")
	}

	// The evidence ledger verifies against the persisted public key.
	pub, err := evidence.LoadPublicKey(KeyPath(base) + ".pub")
	if err != nil {
		t.Fatalf("load pub: %v", err)
	}
	rep, err := evidence.VerifyLedger(EvidenceDir(base), evidence.NewVerifier(map[string]ed25519.PublicKey{KeyID: pub}))
	if err != nil {
		t.Fatalf("verify ledger: %v", err)
	}
	if !rep.OK {
		t.Fatalf("demo ledger must verify, errors: %v", rep.Errors)
	}
	if rep.Count != res.Runs {
		t.Fatalf("ledger receipts %d != runs %d", rep.Count, res.Runs)
	}
}
