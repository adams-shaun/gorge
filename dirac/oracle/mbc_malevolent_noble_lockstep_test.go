package oracle

import "testing"

// Malevolent Noble in hand and Alchemist's Talent on the battlefield: gorge
// offered the cast and the Talent's activation where dirac offered only pass
// (decision 430, tail-16-c000 pair 12).
func TestMBCMalevolentNobleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "malevolent_noble_options")
}
