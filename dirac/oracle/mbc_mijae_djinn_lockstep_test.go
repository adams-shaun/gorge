package oracle

import "testing"

// Mijae Djinn ({R}{R}{R}) was offered to p1 by gorge at a priority decision
// (decision 556 of tail-75-c000 pair 3) where dirac offered only the
// activation of Sigil of Valor.
func TestMBCMijaeDjinnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mijae_djinn_cast_offer")
}
