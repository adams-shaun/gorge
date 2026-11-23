package oracle

import "testing"

// Deathknell Kami's activated pump ("sacrifice it at the beginning of the next
// end step") creates a delayed trigger per activation; with several queued for
// the same end step the controller orders them (CR 603.3b), and both engines
// must pose that ordering decision.
func TestMBCDeathknellKamiReprosLockstep(t *testing.T) {
	stdReproNamed(t, "deathknell_kami_eot_sacrifice_trigger_order")
}
