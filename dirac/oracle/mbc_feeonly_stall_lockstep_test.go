package oracle

import "testing"

// CR 605.3a with CR 601.2g: Bog Initiate's "{1}: Add {B}." is a mana ability
// with no {T} and no limit, so a payment window can activate it again and
// again. Paying Citanul Flute's X=7 with four {B} already floating and a
// generic-only remainder, the window listed it first every time: its fee spent
// the {B} its own activation had just made, so the pool never changed. gorge
// answered the first option forever (a livelock panic at decision 650,
// tail-49 pair 54) and dirac's window walk never ended. A fee-only ability that
// makes no more mana than its fee spends is listed only while its colour fills
// a pip the pool still lacks (gorge feeOnlyConversionStalls, dirac
// windowFeeStalls).
func TestMBCCitanulFluteReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "feeonly_stall_window")
}
