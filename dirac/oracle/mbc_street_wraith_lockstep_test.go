package oracle

import "testing"

// CR 119.4 with CR 702.29a: a player can pay life only if their life total is
// at least the amount, so Street Wraith's "Cycling—Pay 2 life" is no priority
// option for a player at 1 life. gorge withheld the activation; the kernel's
// hand-activation offer checked the mana but not the life part and listed it
// (decision 513, dr-mbc-babb pair 21).
func TestMBCStreetWraithReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "street_wraith_cycling_life_short")
}
