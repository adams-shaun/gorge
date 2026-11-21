package oracle

import "testing"

// Elven Chorus lets p1 cast Wormwood Treefolk ({3}{G}{G}) off the top of the library, with
// five mana sources: two Forests and three creatures the Chorus lets tap for any colour.
// The kernel offered that cast at decision 565 and gorge did not (tail-51-c001, autopay).
func TestMBCWormwoodTreefolkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "wormwood_top_cast_chorus_creature_mana")
}
