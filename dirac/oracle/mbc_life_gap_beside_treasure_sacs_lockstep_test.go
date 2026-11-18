package oracle

import "testing"

// Crimson Fleet Commodore ({3}{R}) in hand beside Treasonous Ogre's life-only
// "Pay 3 life: Add {R}", a Mountain and two fresh Treasure tokens: the three
// plain units fall one short, so gorge's prefix search activates the Ogre and
// the cast is offered. The kernel must offer it too.
func TestMBCCrimsonFleetCommodoreReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_gap_beside_treasure_sacs")
}
