package oracle

import "testing"

// CR 603.3, 605.3b: Bottle-Cap Blast ({4}{R}) at priority with six untapped lands under
// Manabarbs and 4 life. Every land tap is Taps-deferred, so gorge's prefix search taps five
// lands and offers the instant with the five triggers still on the stack; the kernel's plain
// cast gate applied the planner's summed lethal guard in the gaps walk and offered only pass
// (decision 481 of 481; tail-34 pair 118).
func TestMBCBottleCapBlastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "taps_deferred_instant_lethal_prefix")
}
