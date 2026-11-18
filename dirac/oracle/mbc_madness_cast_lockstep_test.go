package oracle

import "testing"

// CR 702.35a: when a card with madness is discarded it is exiled and its owner
// may cast it for the madness cost. Insatiable Gorgers (madness {3}{R}) is
// discarded to hand size; gorge offered the optional "cast it" trigger answers
// yes and no while dirac offered only no (decision 311, pool tail-61-c001).
func TestMBCInsatiableGorgersMadnessOfferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offer")
}
