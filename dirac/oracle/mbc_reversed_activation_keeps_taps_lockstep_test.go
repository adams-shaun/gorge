package oracle

import "testing"

// CR 733.1, 602.2b: gorge reverses A-Riveteers Initiate's {B/G} activation
// when its CR 601.2g window runs dry, and the reversal leaves the mana
// abilities the window activated standing (Chrome Mox and Henge of Ramos stay
// tapped, their mana floats) -- CR 733.1's mana-ability reversal is a "may",
// and gorge does not take it. The kernel's window clamp priced Chrome Mox's
// reflected colours and withheld it (gorge's paymentSourceUseful fails open
// on a ManaReflected ability), so the window was dry at once and the kernel
// reversed without ever tapping anything (tail-26-c000 pair 60, decision 498:
// kernel Cards[43]{Chrome Mox} and Cards[48]{Henge of Ramos} untapped,
// Pool[1] without the {U} and {C} the window floated).
func TestMBCReversedActivationKeepsTapsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reversed_activation_keeps_taps")
}
