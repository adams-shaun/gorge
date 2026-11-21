package oracle

import "testing"

// CR 601.2g / 605.3b: {1}{U}{U}{U} is payable from two Vivid lands' counter
// abilities (one any-colour mana each) plus an Izzet Signet fed by a Forest
// ({1},{T}: {U}{R}). gorge offered the cast of Tempted by the Oriq; the kernel
// did not (tail-47-c001, decision 191).
func TestMBCTemptedByTheOriqReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ctr_gaps_with_signet_fee_cast")
}
