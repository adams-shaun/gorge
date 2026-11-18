package oracle

import "testing"

// CR 605.1a, 601.2g: Treasonous Ogre's "Pay 3 life: Add {R}" is a mana ability
// that helps pay a spell's cost, so {4}{R} Rapacious Dragon is castable from four
// lands and the Ogre. gorge offered the cast and the kernel did not (decision 362
// of dr-mbc-bbbb pair 4).
func TestMBCRapaciousDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_mana_ability_funds_cast_beside_lands")
}
