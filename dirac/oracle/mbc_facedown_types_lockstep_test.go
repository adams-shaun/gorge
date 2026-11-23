package oracle

import "testing"

// CR 708.2 with CR 613.1d: a manifested (face-down) card has no creature types of its own, but
// Mirror Entity's "creatures you control gain all creature types" is a layer-4 effect that applies
// over the face-down values, so the manifested Myriad Landscape is a Zombie. When the Entity's X=0
// kills the board, Archghoul of Thraben ("Zombie you control dies") triggers once for each of the
// four creatures. The kernel read a face-down permanent as having no subtypes whatever the effects
// on it, so it queued three triggers where gorge queued four (decision 405 of 405, tail-16 pair
// 171, autopay).
func TestMBCArchghoulOfThrabenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "facedown_permanent_gains_all_creature_types")
}
