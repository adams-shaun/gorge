package oracle

import "testing"

// Saruli Caretaker ("{T}, tap an untapped creature you control: Add one mana of
// any color") beside Llanowar Envoy (the fee-only repeatable "{1}{G}: Add one
// mana of any color") pays Zombie Apocalypse's {3}{B}{B}{B}: the Caretaker taps
// the Envoy and makes {B}, the Envoy turns its fee into a second {B}. gorge's
// plain prefix search prunes the Caretaker's colour ask to the colours that fill
// a pip still owed (oneShotAskPrune) and finds the line in 35 nodes; the kernel
// walked all five colours of the first tapped creature and spent its 160-node
// budget before it tried the Envoy as the tapped one (tail-28-c000 pair 7,
// decision 339).
func TestMBCZombieApocalypseReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapx_colour_ask_one_shot_prune")
}
