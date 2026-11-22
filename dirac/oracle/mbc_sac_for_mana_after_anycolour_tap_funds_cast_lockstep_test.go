package oracle

import "testing"

// CR 605.3b, CR 601.2g: with Rain of Filth resolved, every land its controller
// has can be sacrificed for {B}, and The World Tree makes each land (six or
// more of them) tap for any color. Tapping the untapped Sunlit Marsh for {G}
// first and then sacrificing lands for the rest pays Puca's Covenant and
// Snarespinner, so priority offers both casts (decision 546, tail-08 pair 13).
// Gorge left both out of the priority menu.
func TestMBCPucaSCovenantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_for_mana_after_anycolour_tap_funds_cast")
}
