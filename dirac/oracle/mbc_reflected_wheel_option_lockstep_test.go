package oracle

import "testing"

// CR 106.6, 107.4e, 602.2b: A-Riveteers Initiate's {B/G}: gains deathtouch,
// with an untapped Plaza of Heroes as p1's only mana source and Nissa, Voice of
// Zendikar (green, legendary) on its battlefield, so Plaza's reflected {G}
// pays the pip. The kernel offered the activation; the autopay lowering of
// gorge's witness answered Plaza's mana-ability wheel with the restricted
// "Add any color" (legendary spells only) instead of the reflected ability,
// the {G} it floated could not pay the pump, and the play was dropped as a
// lowering dead end (decision 450 of tail-26 pair 150, focus A-Riveteers
// Initiate).
func TestMBCReflectedWheelOptionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflected_wheel_option_chosen_by_ability_index")
}
