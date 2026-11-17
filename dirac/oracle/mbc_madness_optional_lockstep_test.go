package oracle

import "testing"

// CR 601.2g, 106.6 with CR 702.35a: Phantasmagorian's "Discard three cards"
// discards Twins of Maurer Estate (madness {2}{B}) to a board of Ancient
// Ziggurat ("{T}: Add one mana of any color. Spend this mana only to cast a
// creature spell") and two Swamps. The Ziggurat's mana can pay the creature
// card's madness cast, so the optional ask offers yes beside no. The kernel
// priced the ask against the unrestricted window only, dropped the Ziggurat and
// offered no alone (first difference decision 185 of tail-23 pair 368, options
// only gorge offering yes(Twins)).
func TestMBCMadnessOptionalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offer_counts_creature_only_mana")
}
