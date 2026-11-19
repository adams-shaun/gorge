package oracle

import "testing"

// CR 106.1a, 605.3b: Three Tree City's {2},{T} makes as much mana of one colour
// as the creatures of its chosen type, so with the type "Tamarian" and no such
// creature it makes none, and a second black source it is not leaves Vampire
// Hexmage ({B}{B}) uncastable beside one Swamp. Gorge didn't offer the cast
// (decision 638, dr-mbc-abbb pair 13); the kernel did.
func TestMBCVampireHexmageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "chosen_type_filter_without_creatures")
}
