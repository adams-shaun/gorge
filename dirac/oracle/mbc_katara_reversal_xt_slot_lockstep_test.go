package oracle

import "testing"

// TestMBCKatarasReversalReprosLockstep replays the tail-62 game whose kernel
// panicked in xtLegalBase (index out of range reading an extra-target slot)
// after three target picks on Katara's Reversal.
func TestMBCKatarasReversalReprosLockstep(t *testing.T) {
	stdReproNamed(t, "katara_reversal_xt_slot_range_panic")
}
