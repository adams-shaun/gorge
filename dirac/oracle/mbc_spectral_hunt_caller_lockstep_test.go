package oracle

import "testing"

// TestMBCSpectralHuntCallerReprosLockstep pins Energy Flux's per-artifact
// "pay {2} or sacrifice it" election on the game where the payer has exactly
// one life and the only tappable mana is River of Tears plus Phyrexian Lens
// ({T}, Pay 1 life). Gorge's paid-cost census counts a life source only while
// its activation leaves the payer alive, so the election offers decline only
// (CR 119.4 permits the payment, but the payer loses before the mana can be
// spent). The kernel's manual window used to tap the Lens anyway and offer a
// pay the gate had already rejected.
func TestMBCSpectralHuntCallerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_lethal_life_payment")
}
