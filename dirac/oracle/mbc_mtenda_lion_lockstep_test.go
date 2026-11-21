package oracle

import "testing"

// Mtenda Lion's "defending player may pay {U}" unless-cost: both engines must
// offer the defender the same pay / don't-pay modes.
func TestMBCMtendaLionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mtenda_lion_unless_payer_modes")
}
