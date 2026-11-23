package oracle

import "testing"

// Dungeon Shade ({3}{B}) in hand beside a Swamp, Prismite ("{2}: Add one mana
// of any color", a repeatable fee-only mana ability) and Treasonous Ogre
// ("Pay 3 life: Add {R}", no {T} and no limit) at 20 life (pool tail-51
// pair 169, surface full, decision 356). Three Ogre activations and the Swamp
// pay the cast (CR 605.3a, 118.3), and gorge offered it while the kernel's
// menu omitted it. Both engines now offer the cast, so the recorded game
// replays clean.
func TestMBCDungeonShadeCastLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sole_life_mana_repeat_beside_fee_filter")
}
