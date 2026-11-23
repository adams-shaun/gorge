package oracle

import "testing"

// TestHybridActivationWindowSourceReprosLockstep replays the recorded
// divergent game: Figure of Fable's {G/W} activation has its witness tap
// Eclipsed Realms, and the {G} the pip does not spend is left floating. It
// stays restricted to the chosen creature type (CR 106.6), so Thoughtweft
// Charge ({1}{G}) is not one of gorge's autopay casts; the kernel must not
// spend the leftover as plain mana and offer the cast.
func TestHybridActivationWindowSourceReprosLockstep(t *testing.T) {
	stdReproNamed(t, "hybrid_activation_window_source")
}
