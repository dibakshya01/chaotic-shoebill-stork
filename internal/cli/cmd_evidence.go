package cli

import (
	"crypto/ed25519"
	"fmt"
	"io"
	"path/filepath"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/evidence"
	"github.com/spf13/cobra"
)

func newEvidenceCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence",
		Short: "Work with the signed evidence ledger",
	}
	cmd.AddCommand(newEvidenceVerifyCmd(stdout, stderr))
	return cmd
}

func newEvidenceVerifyCmd(stdout, stderr io.Writer) *cobra.Command {
	var dir, keyPath, keyID string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify the ledger's signatures, chain, and signed head (tamper/reorder/truncation)",
		Long: "verify confirms every receipt's Ed25519 signature against the public key, that the\n" +
			"hash chain is intact and the sequence is monotonic, and that the signed head matches\n" +
			"the ledger tail (catching tail-truncation). An auditor needs only the public key.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pub, err := evidence.LoadPublicKey(keyPath)
			if err != nil {
				return fmt.Errorf("load public key %s: %w", keyPath, err)
			}
			verifier := evidence.NewVerifier(map[string]ed25519.PublicKey{keyID: pub})
			rep, err := evidence.VerifyLedger(dir, verifier)
			if err != nil {
				return err
			}
			if rep.OK {
				fmt.Fprintf(stdout, "%s✓ ledger verified%s — %d receipts, head seq %d, signatures + chain + head all intact\n",
					cGreen+cBold, cReset, rep.Count, rep.HeadSeq)
				return nil
			}
			fmt.Fprintf(stderr, "%s✗ ledger verification FAILED%s (%d receipts)\n", cRed+cBold, cReset, rep.Count)
			for _, e := range rep.Errors {
				fmt.Fprintf(stderr, "    - %s\n", e)
			}
			return fmt.Errorf("evidence verification failed")
		},
	}
	cmd.Flags().StringVar(&dir, "dir", filepath.Join(".shoebill", "evidence"), "evidence ledger directory")
	cmd.Flags().StringVar(&keyPath, "key", filepath.Join(".shoebill", "evidence.key.pub"), "public key file")
	cmd.Flags().StringVar(&keyID, "key-id", "shoebill-demo", "key id the receipts were signed with")
	return cmd
}
