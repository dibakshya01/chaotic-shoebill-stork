package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/score"
)

// ANSI colors (disabled when NO_COLOR is set or output is not a terminal-ish stream).
var (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cGreen  = "\033[32m"
	cAmber  = "\033[33m"
	cRed    = "\033[31m"
	cRedBg  = "\033[41;97m"
	cCyan   = "\033[36m"
	cGrayFg = "\033[90m"
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		cReset, cBold, cDim, cGreen, cAmber, cRed, cRedBg, cCyan, cGrayFg = "", "", "", "", "", "", "", "", ""
	}
}

// familyCols is the fixed column order for the matrix.
var familyCols = []model.Family{
	model.FamilyMesh, model.FamilyGateway, model.FamilyIdentity, model.FamilyNetwork,
	model.FamilyData, model.FamilySupplyChain, model.FamilyDR, model.FamilyDetection, model.FamilyAI,
}

var famShort = map[model.Family]string{
	model.FamilyMesh: "Mesh", model.FamilyGateway: "Gwy", model.FamilyIdentity: "Idnt",
	model.FamilyNetwork: "Netw", model.FamilyData: "Data", model.FamilySupplyChain: "Supl",
	model.FamilyDR: "DR", model.FamilyDetection: "Detc", model.FamilyAI: "AI",
}

func glyph(state string) string {
	switch state {
	case "pass":
		return cGreen + "●" + cReset
	case "slow":
		return cAmber + "◐" + cReset
	case "fail":
		return cRed + "▲" + cReset
	case "fail_open":
		return cRedBg + "✖" + cReset
	default: // untested
		return cGrayFg + "○" + cReset
	}
}

// RenderScorecard prints the posture scorecard matrix + summary to w.
func RenderScorecard(w io.Writer, sc score.Scorecard) {
	s := sc.Summary
	fmt.Fprintf(w, "\n%s%s  Security Control Posture%s\n", cBold, cCyan, cReset)
	fmt.Fprintf(w, "%s  ────────────────────────────────────────────────────────%s\n", cDim, cReset)
	fmt.Fprintf(w, "  Efficacy %s%.0f%%%s   Fail-open %s%d open%s   Coverage %d/%d   Freshness %.0f%%\n\n",
		cBold, s.Efficacy*100, cReset, redIf(s.FailOpenOpen), s.FailOpenOpen, cReset,
		s.CoverageTested, s.CoverageTotal, s.Freshness*100)

	// Which columns to show: families that appear in any service's expected set.
	show := map[model.Family]bool{}
	for _, card := range sc.Services {
		for fam := range card.Cells {
			show[fam] = true
		}
	}
	// Header.
	fmt.Fprintf(w, "  %-18s", "")
	for _, fam := range familyCols {
		if show[fam] {
			fmt.Fprintf(w, " %-5s", famShort[fam])
		}
	}
	fmt.Fprintf(w, "  %s\n", "Score")
	// Rows.
	for _, card := range sc.Services {
		tier := fmt.Sprintf("T%d", card.Service.Tier)
		fmt.Fprintf(w, "  %-14s %s%-3s%s", card.Service.Name, cDim, tier, cReset)
		for _, fam := range familyCols {
			if !show[fam] {
				continue
			}
			cell, ok := card.Cells[fam]
			if !ok {
				fmt.Fprintf(w, " %-5s", " ")
				continue
			}
			fmt.Fprintf(w, "   %s  ", glyph(cell.State))
		}
		fmt.Fprintf(w, "  %s%.2f%s\n", scoreColor(card.Score), card.Score, cReset)
	}
	fmt.Fprintf(w, "\n  %s●%s pass  %s◐%s slow  %s○%s untested  %s▲%s fail  %s✖%s fail-open\n",
		cGreen, cReset, cAmber, cReset, cGrayFg, cReset, cRed, cReset, cRedBg, cReset)
}

func scoreColor(v float64) string {
	switch {
	case v >= 0.8:
		return cGreen
	case v >= 0.5:
		return cAmber
	default:
		return cRed
	}
}

func redIf(n int) string {
	if n > 0 {
		return cRed + cBold
	}
	return cGreen
}

// RenderFailOpen prints the fail-open findings, the platform's reason to exist.
func RenderFailOpen(w io.Writer, lines []string) {
	if len(lines) == 0 {
		fmt.Fprintf(w, "\n  %sNo open fail-open findings.%s\n", cGreen, cReset)
		return
	}
	fmt.Fprintf(w, "\n%s%s  ✖ FAIL-OPEN FINDINGS%s  %s(a control degraded permissively — highest severity)%s\n",
		cBold, cRed, cReset, cDim, cReset)
	for _, l := range lines {
		fmt.Fprintf(w, "  %s\n", l)
	}
}

func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}
