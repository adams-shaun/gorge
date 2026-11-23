package oracle

import "testing"

// tail-51-c000 pair 52 (seed 883197): at a p0 priority decision with one
// untapped Plains and a Treasonous Ogre ("Pay 3 life: Add {R}") on the
// battlefield, gorge offered cast(Heiko Yamazaki, the General) ({3}{R}) while
// dirac offered only the other casts and pass.
func TestMBCHeikoYamazakiTheGeneralReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paylife_mana_unit_repeats")
}
