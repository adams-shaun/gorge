package oracle

import "testing"

// CR 605.3a, 119.4: Treasonous Ogre's "Pay 3 life: Add {R}" taps nothing, so
// it is activated as often as the payer's life allows. With two untapped
// Plains, Prismite's open "{2}: Add one mana of any color" on the board (it
// makes gorge's PotentialMana bound unbounded) and 18 life, gorge's prefix
// search floats {W}{W} and three {R} from the Ogre (9 life) and offers
// Noble Purpose ({3}{W}{W}). The dirac search priced a life-only activation
// as no gap, so it never offered the cast after the activations (pair 27 of
// tail-51, decision 532).
func TestMBCNoblePurposeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_only_mana_repeats_beside_a_filter_gap")
}
