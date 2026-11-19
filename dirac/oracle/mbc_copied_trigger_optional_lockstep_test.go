package oracle

import "testing"

// CR 603.5 with CR 113.7a / 603.10a: Malleable Impostor entered as a copy of
// Hollowsage, its "whenever this becomes untapped, you may have target player
// discard a card" trigger went on the stack, and Ominous Cemetery shuffled the
// Impostor into its owner's library in response. The ability is still the
// copy's: its controller is asked the "may" at resolution. gorge once lost the
// trigger line with the copy, skipped the question and went straight to the
// discard (decision 412 of the dealt game, tail-68-c002).
func TestMBCOminousCemeteryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copied_trigger_optional_after_copy_left")
}
