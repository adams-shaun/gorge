package oracle

import "testing"

// CR 601.2b, 605.3b: a counter-cost gap (Vivid Marsh, Vivid Grove) beside a fee filter (Izzet Signet's
// {1},{T}: {U}{R}) on a board of five Islands. gorge's plain prefix run spends its whole 160-node budget
// walking the fee's branches and finds no script; its fee-order run (covered funding first, then the
// fee-carrying gaps, then the free ones) pays {B}{B} for A-The Meathook Massacre and {2}{B}{B} for
// Phyrexian Defiler in 15 nodes. The kernel declined the fee-order run whenever any other gap was on the
// board and listed neither cast (decision 323 of 323, dr tail-47 pair 3).
func TestMBCATheMeathookMassacreReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ctr_gap_beside_fee_filter_fee_order")
}
