package oracle

import "testing"

// CR 611.2b with 509.1b: Sly Instigator's "target creature can't be blocked
// this turn" Effect runs Duration$ UntilYourNextTurn, so a creature it
// targeted on its controller's turn is still unblockable on the opponent's
// following turn. Chromeshell Crab, made unblockable that way, attacked into
// Freewind Falcon: gorge offered no block of the Crab, while the kernel let the
// grant lapse at the end of the turn it was made and offered Falcon blocking it
// (synth tail-45 pair 13, seed 9798527289607150121, decision 750).
func TestMBCFreewindFalconReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sly_instigator_unblockable_until_next_turn")
}
