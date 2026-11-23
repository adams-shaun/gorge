package oracle

import "testing"

// Connive // Concoct's Concoct face cast by autopay with Bog Initiate's
// "{1}: Add {B}" conversion on the battlefield (tail-49-c000 pair 5, decision
// 407): gorge and dirac must tap the same sources and leave the same pool.
func TestMBCConniveReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phased_fallback_beside_taps_deferred_gap")
}
