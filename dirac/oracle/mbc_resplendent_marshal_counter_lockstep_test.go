package oracle

import "testing"

// CR 205.3m: Resplendent Marshal puts a +1/+1 counter only on each other
// creature you control that shares a creature type with the exiled card.
// The exiled Briarbridge Tracker (Human Scout) shares nothing with the
// Marshal already on the battlefield (Angel Warrior), yet gorge put the
// counter on it and the kernel (rightly) did not (tail-20 pair 19, decision
// 750: projection Cards[70].Power).
func TestMBCResplendentMarshalCounterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "marshal_counter_needs_shared_creature_type")
}
