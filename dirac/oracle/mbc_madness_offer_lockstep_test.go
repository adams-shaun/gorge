package oracle

import "testing"

// Twins of Maurer Estate (Madness {2}{B}) discarded to Tortured Existence: the
// cast-or-graveyard ask must offer "yes" exactly when gorge does (decision 704 of
// tail-23-c000 pair 3; gorge offered yes and no, the kernel only no).
func TestMBCTwinsOfMaurerEstateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offer_affordability")
}
