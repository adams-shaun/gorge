package oracle

import "testing"

// CR 605.3a, 601.2g-h, 602.2b: p1's Retrofitter Foundry is tapped with Hickory
// Woodlot and Viridescent Bog untapped. Gorge's prefix search taps the
// Woodlot's {G}{G} (a depletion-counter census gap), one {G} pays the Bog's
// {1} fee, and the Bog's {B}{G} completes the Foundry's {3} untap ability, so
// it is offered; dirac offered only pass (decision 340 of 340, tail-12 pair 68).
func TestMBCRetrofitterFoundryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "depletion_gap_beside_filter_pays_untap_activation")
}
