package oracle

import "testing"

// CR 605.3a, 601.2g-h, 118.3: Heap Gate's "{1}, {T}: Add one mana of any color"
// is funded by Treasonous Ogre's "Pay 3 life: Add {R}" (the Ogre's only mana
// ability), and the any-color mana pays Dungeon Shade's "{B}: +1/+1" pump.
// gorge offered the activation and dirac didn't (tail-51 pair 128, decision 494).
func TestMBCDungeonShadeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sole_life_source_funds_filter_fee_for_ability")
}
