package evidence

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Signer produces detached Ed25519 signatures over receipts.
type Signer struct {
	keyID string
	priv  ed25519.PrivateKey
	pub   ed25519.PublicKey
	now   func() time.Time
}

// SignerOption customizes a Signer (used to inject a deterministic clock in tests).
type SignerOption func(*Signer)

// WithClock injects the time source used for SignedAt.
func WithClock(now func() time.Time) SignerOption {
	return func(s *Signer) { s.now = now }
}

// NewSigner builds a Signer from a private key.
func NewSigner(keyID string, priv ed25519.PrivateKey, opts ...SignerOption) *Signer {
	s := &Signer{
		keyID: keyID,
		priv:  priv,
		pub:   priv.Public().(ed25519.PublicKey),
		now:   time.Now,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// PublicKey returns the signer's public key (for distribution to verifiers/auditors).
func (s *Signer) PublicKey() ed25519.PublicKey { return s.pub }

// KeyID returns the signer's key identifier.
func (s *Signer) KeyID() string { return s.keyID }

// Sign computes the receipt's content hash and attaches a signature. The Signature
// field is excluded from the content it signs, so re-signing is idempotent.
func (s *Signer) Sign(r *Receipt) error {
	if s.priv == nil {
		return errors.New("signer has no private key")
	}
	digest, hexDigest, err := contentHash(r)
	if err != nil {
		return err
	}
	sig := ed25519.Sign(s.priv, digest)
	r.Signature = &Signature{
		Alg:         "Ed25519",
		KeyID:       s.keyID,
		ContentHash: hexDigest,
		Value:       base64.StdEncoding.EncodeToString(sig),
		SignedAt:    s.now().UTC(),
	}
	return nil
}

// contentHash returns the sha256 digest (and its hex form) of the canonical receipt
// with its Signature field cleared. Canonicalization is deterministic: struct field
// order is fixed and encoding/json sorts map keys.
func contentHash(r *Receipt) ([]byte, string, error) {
	clone := *r
	clone.Signature = nil
	b, err := json.Marshal(&clone)
	if err != nil {
		return nil, "", fmt.Errorf("canonicalize receipt: %w", err)
	}
	sum := sha256.Sum256(b)
	return sum[:], hex.EncodeToString(sum[:]), nil
}

// Verifier checks receipt signatures against a set of trusted public keys.
type Verifier struct {
	keys map[string]ed25519.PublicKey
}

// NewVerifier builds a Verifier trusting the given keyID -> public key map.
func NewVerifier(keys map[string]ed25519.PublicKey) *Verifier {
	return &Verifier{keys: keys}
}

// key returns the trusted public key for a key id, if present.
func (v *Verifier) key(id string) (ed25519.PublicKey, bool) {
	pub, ok := v.keys[id]
	return pub, ok
}

// ErrUntrustedKey is returned when a receipt is signed by an unknown key.
var ErrUntrustedKey = errors.New("receipt signed by an untrusted key id")

// ErrBadSignature is returned when the signature does not verify (tampering).
var ErrBadSignature = errors.New("receipt signature does not verify (content was altered or forged)")

// Verify confirms the receipt was signed by a trusted key and not altered since.
func (v *Verifier) Verify(r *Receipt) error {
	if r.Signature == nil {
		return errors.New("receipt is unsigned")
	}
	pub, ok := v.keys[r.Signature.KeyID]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUntrustedKey, r.Signature.KeyID)
	}
	digest, hexDigest, err := contentHash(r)
	if err != nil {
		return err
	}
	// The embedded content hash must match the recomputed one...
	if hexDigest != r.Signature.ContentHash {
		return fmt.Errorf("%w: content hash mismatch", ErrBadSignature)
	}
	// ...and the signature must verify over that digest.
	sig, err := base64.StdEncoding.DecodeString(r.Signature.Value)
	if err != nil {
		return fmt.Errorf("%w: signature not base64: %v", ErrBadSignature, err)
	}
	if !ed25519.Verify(pub, digest, sig) {
		return ErrBadSignature
	}
	return nil
}

// --- key material helpers ---------------------------------------------------

// GenerateKey creates a fresh Ed25519 keypair.
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// LoadOrCreateKey loads an Ed25519 private key from path (base64 seed), creating a
// new keypair (and writing the public key to path+".pub") if the file is absent.
// The private key file is written with 0600 permissions.
func LoadOrCreateKey(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		seed, derr := base64.StdEncoding.DecodeString(trimSpace(string(b)))
		if derr != nil {
			return nil, fmt.Errorf("decode key %s: %w", path, derr)
		}
		if len(seed) != ed25519.SeedSize {
			return nil, fmt.Errorf("key %s: bad seed length %d", path, len(seed))
		}
		return ed25519.NewKeyFromSeed(seed), nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read key %s: %w", path, err)
	}
	// Create.
	pub, priv, gerr := GenerateKey()
	if gerr != nil {
		return nil, gerr
	}
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o700); mkErr != nil {
		return nil, mkErr
	}
	seed := priv.Seed()
	if werr := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(seed)+"\n"), 0o600); werr != nil {
		return nil, fmt.Errorf("write key %s: %w", path, werr)
	}
	if werr := os.WriteFile(path+".pub", []byte(base64.StdEncoding.EncodeToString(pub)+"\n"), 0o644); werr != nil {
		return nil, fmt.Errorf("write pubkey: %w", werr)
	}
	return priv, nil
}

// LoadPublicKey reads a base64-encoded Ed25519 public key from path.
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(trimSpace(string(b)))
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("bad public key length %d", len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

func trimSpace(s string) string {
	// minimal trim to avoid importing strings for one call path churn
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	return s
}
