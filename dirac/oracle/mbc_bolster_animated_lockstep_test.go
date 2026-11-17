package oracle

import "testing"

// CR 701.39a with CR 205.1b/613.1d: Honor's Reward bolsters 2 among the
// creatures its controller has with the least toughness, and a permanent that
// has become a creature (Opal Champion, animated by an opposing creature
// spell) counts. Its 3/3 ties the Ruin Crabs' 0/3, so the tie election offers
// it too. Gorge read the printed type line and left the animated enchantment
// out of the election (decision 672 of tail-33-c002, pair 6).
func TestMBCOpalChampionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bolster_counts_animated_creature")
}
