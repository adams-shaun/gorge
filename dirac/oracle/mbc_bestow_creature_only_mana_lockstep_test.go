package oracle

import "testing"

// CR 702.103b with CR 106.6: Nimbus Naiad's bestowed cast is an Aura spell, not
// a creature spell, so Beastcaller Savant's mana ("spend this mana only to cast
// a creature spell") can't pay its bestow cost {4}{U}. With three Forests,
// Slayers' Stronghold and the Savant untapped, the Savant is p1's only blue
// source: gorge offers the plain {2}{U} cast and not the bestowed one, which
// the kernel once offered by reading the restriction against the printed
// creature card (decision 313 of 313; tail-30-c000 pair 108, seed 242677).
func TestMBCNimbusNaiadReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bestow_creature_only_mana")
}
