package oracle

import "testing"

// Azorius Locket's {0(W/U)(W/U)(W/U)(W/U)}, {T}, Sacrifice this artifact:
// Draw a card activation (seed 50, autopay): the manual CR 601.2g window
// settles all four hybrid pips to {U}, and when the walk's sources run out
// one {U} is still owed. gorge aborts the activation and keeps the window's
// taps and floating mana (CR 601.2g, 733.1), so the kernel must reverse the
// activation rather than plan the unpayable remainder.
func TestMBCHybridActivationWindowDryReprosLockstep(t *testing.T) {
	stdReproNamed(t, "hybrid_activation_window_dry")
}
