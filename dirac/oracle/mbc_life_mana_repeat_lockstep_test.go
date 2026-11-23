package oracle

import "testing"

// CR 605.3b, 118.3: Treasonous Ogre's "Pay 3 life: Add {R}" has no {T} or other
// activation limit, so a payment may activate it as often as the payer's life
// funds it. With a Turbulent Shore, Swamp, Plains and Mountain beside an Ogre
// (and Prismite) gorge offered Edgar Markov ({3}{B}{R}{W}) from hand: the three
// colour pips from the Swamp, Plains and Mountain, the generic from the Shore
// and two Ogre activations. The kernel counted the Ogre once and left the cast
// out of the priority menu (decision 226, tail-51 pair 167).
func TestMBCEdgarMarkovOgreRepeatReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_only_mana_ability_repeats_for_cast")
}
