package oracle

import "testing"

// CR 602.2b / 605.3a: The Surgical Bay's "{1}{U}, {T}, Sacrifice The Surgical
// Bay: Draw a card." beside Henge of Ramos ("{2}, {T}: Add one mana of any
// color") and a row of identical Swamps. The fee-carrying gap's branch needs
// the indistinguishable-source cut the fee-only path already took: without it
// the prefix search budget-limited at 160 clones and the payable draw stayed
// census_incomplete at every priority (tail-26 pair 26: 203 searches, about
// 16s of a 2-seat game). The cut in both engines prices the draw and keeps
// the game in lockstep.
func TestMBCTail26ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "surgical_bay_henge_fee_twin")
}
