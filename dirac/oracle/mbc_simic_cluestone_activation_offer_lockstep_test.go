package oracle

import "testing"

// At p1's priority (tail-45 pair 151, decision 1094) gorge offered Simic
// Cluestone's {G}{U},{T},sac draw, while the kernel offered only pass. The
// fee-only any-colour filter (Signpost Scarecrow) funds its next activation
// from the pool, so gorge's prefix search walks a chain longer than sixteen
// steps; the kernel's filter search capped a path at sixteen and dropped that
// branch. The recording replays on its own decks' table.
func TestMBCSimicCluestoneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "simic_cluestone_activation_offer")
	// tail-45 pair 80, decision 548: the opposite split. The kernel offered the
	// same draw and gorge offered only pass.
	stdReproNamed(t, "mana_filter_twice_funds_activation")
}
