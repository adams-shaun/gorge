package oracle

import "testing"

const feeOrderSecondFilterRepro = "fee_order_run_second_filter_gap_keeps_dead_free_branch"

// CR 601.2g, 605.3a-b: Captain Marvel's {3}{W}{W} beside Baxter Building's
// "{4},{T}: Add four mana in any combination of colors" and Surveillance Room's
// {1},{T} any-colour filter (two filter gaps) is funded by gorge's fee-order run:
// its plain run walks Baxter's free {C} branch, a dead one, until the node budget
// is spent, so the script funds the fee from the covered lands (Hidden Lair among
// them) and leaves Surveillance Room untapped. The kernel's free-branch prune
// (freeBranchHopeless) is the lone filter's model (Vault of the Archangel); on a
// second filter gap it took the plain run's script and tapped Surveillance Room
// instead of Hidden Lair (std-msh, focus Captain Marvel beside Baxter Building
// and Hidden Lair, pair 268, seed 30698).
func TestFeeOrderSecondFilterGapKeepsDeadFreeBranch(t *testing.T) {
	res := replayRawRepro(t, feeOrderSecondFilterRepro)
	if res.Div != nil {
		t.Fatalf("%s diverges at %d: %s", feeOrderSecondFilterRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s ends at an unmodelled stall: %q", feeOrderSecondFilterRepro, res.Stall)
	}
}
