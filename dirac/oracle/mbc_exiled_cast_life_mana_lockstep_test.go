package oracle

import "testing"

// CR 601.2g, 118.3, 119.4: Flames of the Firebrand ({2}{R}) was exiled by a
// Junk token this turn and is playable; the caster has one untapped Mountain
// and Treasonous Ogre ("Pay 3 life: Add {R}"), so gorge's planner pays the
// other two pips with two Ogre activations and offers the cast. dirac must
// offer the same cast on the exiled card.
func TestMBCFlamesOfTheFirebrandReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exiled_cast_two_life_mana")
}
