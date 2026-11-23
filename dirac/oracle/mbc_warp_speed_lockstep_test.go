package oracle

import "testing"

const warpSpeedRepro = "filter_five_unit_fee_script_budget_exhausted"

// CR 605.3b, 602.2b: Warp Speed's activation (reduced to {2}{U}{U} plus
// sacrificing itself) is payable only through Starfleet Academy's
// "{5}, {T}: Add X mana of one colour" filter, fed by five other lands.
// gorge's prefix search walks the filter's five-unit fee through every
// land's colour ask and exhausts its 160-node budget (plain and phased)
// with no script, so it offers pass alone at decision 681 (std-trk pair
// 128, seed 697578); the kernel's filter search used to fund the same line
// in 9 nodes and offer both activations. freeBranchHopeless now keeps the
// free branch when the filter's production amount is a dynamic one: a
// dynamic amount builds no fee-ordered run (potentialPrefixScript's
// feeGaps read takes availableAmount, which reads a dynamic amount as
// zero), so the dead subtree under the free tap is gorge's plain run to
// walk and its node budget dies there, as this search's walk now does too.
// On the pinned game both engines offer pass alone, the recorded game
// replays clean, and the card is no longer refused
// (dirac/support/unrefused.tsv); this test pins the unrefusal's proof.
func TestWarpSpeedFilterFeeBudgetReproReplaysClean(t *testing.T) {
	res := replayRawRepro(t, warpSpeedRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", warpSpeedRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", warpSpeedRepro, res.Stall)
	}
}
