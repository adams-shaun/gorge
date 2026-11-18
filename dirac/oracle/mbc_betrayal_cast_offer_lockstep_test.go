package oracle

import "testing"

// Betrayal's cast offer (CR 601.2) after the priority decision: gorge offers
// the cast beside Cursed Totem, so the priority options must agree. Recorded
// from tail-74 pair 2 at seed 108741, decision 319.
func TestMBCBetrayalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "betrayal_cast_offer")
}
