package oracle

import "testing"

// Glimmerbell's "{1}{U}: Untap Glimmerbell" is offered by gorge with the
// first activation still on the stack and only Hidden Hideout (carrying the
// opponent's Weirding Wood) and two Swamps untapped, and the kernel offers
// only pass (decision 738; the tail-20 Glimmerbell focus sweep, pair 365).
func TestMBCGlimmerbellReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "untap_self_activation_funded_by_an_opponents_aura_grant")
}
