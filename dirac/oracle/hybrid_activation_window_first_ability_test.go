package oracle

import "testing"

// TestHybridActivationWindowFirstAbilityReprosLockstep replays Creakwood
// Ghoul's hybrid {B/G}{B/G} activation over a {B}{G} pool: gorge arms no
// witness for a hybrid-only activation (rules armAbilityAutoPay's
// abilityCostNeedsMana prices no generic or coloured pip in a cost of hybrid
// pips alone), so the pips' announced {G}{G} leaves its ordinary manual
// CR 601.2g window open, and the window's clamp keeps Vivid Meadow listed by
// its counter ability while the answer takes the source's plain first {T} —
// a {W} floats before Forest's {G} pays. The kernel's planner-witness route
// (the pips' resolution the sources afford) skipped Vivid Meadow entirely.
func TestHybridActivationWindowFirstAbilityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_activation_window_first_ability")
}
