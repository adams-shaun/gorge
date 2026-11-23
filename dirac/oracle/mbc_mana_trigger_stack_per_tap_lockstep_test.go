package oracle

import "testing"

// CR 603.3b with CR 605.3b: Bottle-Cap Blast is cast with Manabarbs on both
// sides, paid by scripted taps. gorge answers each tap as its own decision, so
// each tap's two triggers go on the stack (the active player's first) before
// the next tap; dirac stacked them in one batch after the last tap.
func TestMBCBottleCapBlastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_trigger_stack_per_tap_prefix")
}
