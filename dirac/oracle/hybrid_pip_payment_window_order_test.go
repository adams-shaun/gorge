package oracle

import "testing"

// Architects of Will's Cycling {U/B}, activated from hand (a keyword-granted
// activation), floats the cost's mana before the hybrid pip's choose:pay is
// posed (CR 601.2g, 107.4e): gorge taps a Swamp and holds {B}, and the kernel
// must hold the same mana at that ask rather than tapping after the answer.
// The recording is kfuzz -synth tail-10 -surface autopay -mulligans 0, seed 1,
// pair 110 (272 of ~2318 games of that scan diverged at this shape).
func TestMBCHybridPipPaymentWindowOrderReprosLockstep(t *testing.T) {
	stdReproNamed(t, "hybrid_pip_payment_window_order")
}
