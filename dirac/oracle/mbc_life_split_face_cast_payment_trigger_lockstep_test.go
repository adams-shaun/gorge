package oracle

import "testing"

// CR 601.2a with CR 709.3: casting the Death half of Life // Death from the
// hand (a sorcery) requires an empty stack, and the mana ability that pays it
// taps an Island, so the payment's Scald trigger must resolve before the
// spell is put on the stack. The kernel left Scald on the stack above the
// Death spell. Decision 642 of 642.
func TestMBCLifeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_split_face_cast_payment_trigger")
}
