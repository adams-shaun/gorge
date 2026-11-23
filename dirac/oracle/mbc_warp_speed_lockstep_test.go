package oracle

import "testing"

const warpSpeedRepro = "filter_five_unit_fee_script_budget_exhausted"

// CR 605.3b, 602.2b: Warp Speed's activation (reduced to {2}{U}{U} plus
// sacrificing itself) is payable only through Starfleet Academy's
// "{5}, {T}: Add X mana of one colour" filter, fed by five other lands.
// Gorge's prefix search walks the filter's five-unit fee through every land's
// colour ask and exhausts its 160-node budget (plain and phased) with no
// script, so it offers pass alone at decision 681 (std-trk pair 128, seed
// 697578). The kernel's filter search used to fund the line in 9 nodes: its
// freeBranchHopeless prune (#2670) skipped the dead free branches gorge walks.
// The prune now fails open beside a dynamic-amount filter, so both engines
// offer pass alone, Warp Speed is no longer refused
// (dirac/support/unrefused.tsv) and the recorded game replays in lockstep.
func TestMBCWarpSpeedReprosLockstep(t *testing.T) {
	t.Parallel()
	res := replayRawRepro(t, warpSpeedRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", warpSpeedRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", warpSpeedRepro, res.Stall)
	}
}
