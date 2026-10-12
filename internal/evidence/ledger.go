package evidence

import (
	"bufio"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	ledgerFile = "ledger.jsonl"
	headFile   = "head.json"
)

// Head is the separately-stored, signed high-water mark of the ledger. It records
// the sequence number and content hash of the last receipt, signed by the ledger's
// key. Because it is stored and signed independently of the chain, dropping the last
// N receipts (tail truncation) — which a bare hash chain cannot detect — is caught:
// the surviving chain's tail no longer matches the signed head.
type Head struct {
	Seq      int64     `json:"seq"`
	Hash     string    `json:"hash"`
	Count    int64     `json:"count"`
	KeyID    string    `json:"key_id"`
	SignedAt time.Time `json:"signed_at"`
	Sig      string    `json:"sig,omitempty"`
}

func (h Head) signingBytes() ([]byte, error) {
	clone := h
	clone.Sig = ""
	return json.Marshal(&clone)
}

// Ledger is an append-only, hash-chained, signed receipt store backed by a directory
// (a JSONL chain file + a signed head file).
type Ledger struct {
	dir      string
	signer   *Signer
	mu       sync.Mutex
	lastSeq  int64
	lastHash string
	count    int64
}

// OpenLedger opens (creating if needed) a ledger directory, loading existing state so
// that appends continue the chain. The signer signs both receipts and the head.
func OpenLedger(dir string, signer *Signer) (*Ledger, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	l := &Ledger{dir: dir, signer: signer, lastSeq: -1}
	receipts, err := readReceipts(filepath.Join(dir, ledgerFile))
	if err != nil {
		return nil, err
	}
	if n := len(receipts); n > 0 {
		last := receipts[n-1]
		_, h, herr := contentHash(last)
		if herr != nil {
			return nil, herr
		}
		l.lastSeq = last.Seq
		l.lastHash = h
		l.count = int64(n)
	}
	return l, nil
}

// Append assigns the next sequence number, chains to the previous receipt, signs the
// receipt, writes it, and updates the signed head. It is safe for concurrent use.
func (l *Ledger) Append(r *Receipt) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	r.Seq = l.lastSeq + 1
	r.PrevHash = l.lastHash
	if r.SchemaVersion == "" {
		r.SchemaVersion = SchemaVersion
	}
	if err := l.signer.Sign(r); err != nil {
		return err
	}
	_, h, err := contentHash(r)
	if err != nil {
		return err
	}

	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(l.dir, ledgerFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	head := Head{Seq: r.Seq, Hash: h, Count: l.count + 1, KeyID: l.signer.KeyID(), SignedAt: l.signer.now().UTC()}
	if err := l.writeHead(head); err != nil {
		return err
	}

	l.lastSeq = r.Seq
	l.lastHash = h
	l.count++
	return nil
}

func (l *Ledger) writeHead(h Head) error {
	sb, err := h.signingBytes()
	if err != nil {
		return err
	}
	h.Sig = base64.StdEncoding.EncodeToString(ed25519.Sign(l.signerPriv(), sb))
	b, err := json.MarshalIndent(&h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(l.dir, headFile), b, 0o600)
}

// signerPriv reaches the signer's private key for head signing (same key as receipts).
func (l *Ledger) signerPriv() ed25519.PrivateKey { return l.signer.priv }

// Receipts returns all receipts in the ledger, in order.
func (l *Ledger) Receipts() ([]*Receipt, error) {
	return readReceipts(filepath.Join(l.dir, ledgerFile))
}

func readReceipts(path string) ([]*Receipt, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []*Receipt
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Receipt
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("parse receipt: %w", err)
		}
		cp := r
		out = append(out, &cp)
	}
	return out, sc.Err()
}

// LedgerReport is the result of verifying a ledger directory.
type LedgerReport struct {
	Dir     string   `json:"dir"`
	Count   int      `json:"count"`
	OK      bool     `json:"ok"`
	Errors  []string `json:"errors,omitempty"`
	HeadSeq int64    `json:"head_seq"`
}

// VerifyLedger checks every receipt signature, the chain linkage, sequence
// monotonicity, and the signed head (which catches tail truncation).
func VerifyLedger(dir string, verifier *Verifier) (LedgerReport, error) {
	rep := LedgerReport{Dir: dir, OK: true, HeadSeq: -1}
	receipts, err := readReceipts(filepath.Join(dir, ledgerFile))
	if err != nil {
		return rep, err
	}
	rep.Count = len(receipts)

	var prevHash string
	for i, r := range receipts {
		// signature
		if err := verifier.Verify(r); err != nil {
			rep.fail(fmt.Sprintf("receipt %d (%s): %v", i, r.ReceiptID, err))
		}
		// sequence monotonicity (contiguous from 0)
		if r.Seq != int64(i) {
			rep.fail(fmt.Sprintf("receipt %d: seq %d out of order (expected %d)", i, r.Seq, i))
		}
		// chain linkage
		if r.PrevHash != prevHash {
			rep.fail(fmt.Sprintf("receipt %d (%s): prev_hash mismatch — chain broken (reorder/removal)", i, r.ReceiptID))
		}
		_, h, herr := contentHash(r)
		if herr != nil {
			return rep, herr
		}
		prevHash = h
	}

	// Head check (truncation detection).
	head, herr := readHead(filepath.Join(dir, headFile))
	switch {
	case herr != nil && !os.IsNotExist(herr):
		return rep, herr
	case herr != nil: // no head file
		rep.fail("no signed head file present (cannot rule out tail truncation)")
	default:
		rep.HeadSeq = head.Seq
		if verr := verifyHead(head, verifier); verr != nil {
			rep.fail(fmt.Sprintf("head signature: %v", verr))
		}
		if len(receipts) == 0 {
			rep.fail("head present but ledger is empty")
		} else {
			last := receipts[len(receipts)-1]
			_, lh, _ := contentHash(last)
			if head.Seq != last.Seq || head.Hash != lh {
				rep.fail(fmt.Sprintf("head (seq=%d) does not match ledger tail (seq=%d) — TAIL TRUNCATION or tampering", head.Seq, last.Seq))
			}
			if head.Count != int64(len(receipts)) {
				rep.fail(fmt.Sprintf("head count %d != ledger length %d", head.Count, len(receipts)))
			}
		}
	}
	return rep, nil
}

func (r *LedgerReport) fail(msg string) {
	r.OK = false
	r.Errors = append(r.Errors, msg)
}

func readHead(path string) (Head, error) {
	var h Head
	b, err := os.ReadFile(path)
	if err != nil {
		return h, err
	}
	return h, json.Unmarshal(b, &h)
}

func verifyHead(h Head, verifier *Verifier) error {
	pub, ok := verifier.key(h.KeyID)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUntrustedKey, h.KeyID)
	}
	if h.Sig == "" {
		return errors.New("head is unsigned")
	}
	sb, err := h.signingBytes()
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(h.Sig)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadSignature, err)
	}
	if !ed25519.Verify(pub, sb, sig) {
		return ErrBadSignature
	}
	return nil
}
