package oracle

import "testing"

// TestCmd5OrbFeeGapAutopayOffer: at a priority ask gorge offers Lathliss,
// Dragon Queen's {1}{R} pump, a play only a funded fee gap pays (the Orb of
// Dragonkind's {1},{T}, its fee from the pool). Gorge's census-gap arm runs
// on the autopay surface too (autoPayOffer, potentialCensusFallback); the
// kernel's afford checks used to price no fee unit there and under-offered
// (CR 601.2g, 605.3b).
func TestCmd5OrbFeeGapAutopayOffer(t *testing.T) {
	t.Parallel()
	res := replayCmd5ReproUnverified(t, "cmd5_orb_fee_gap_autopay_offer", 834, "options")
	requireCmd5ReproClean(t, res, 834)
}

// TestCmd5OrbFeeGapAutopayMayPlayOffer: gorge offers a may-play cast of
// Thunderbreak Regent from exile, a play only a funded fee gap pays; the
// kernel's may-play afford used to price no fee unit on the autopay surface
// and under-offered (CR 601.2b, 605.3b).
func TestCmd5OrbFeeGapAutopayMayPlayOffer(t *testing.T) {
	t.Parallel()
	res := replayCmd5ReproUnverified(t, "cmd5_orb_fee_gap_autopay_mayplay_offer", 743, "options")
	requireCmd5ReproClean(t, res, 743)
}
