package oracle

import "testing"

// TestCmd5OrbFeeGapSolRingFunding: a {R} pump nothing but a Sol Ring and an
// Orb of Dragonkind's restricted production pays. Gorge's script funds the
// Orb's {1} fee from the first covered source in battlefield order, the Sol
// Ring's {C}{C}, whatever colour the cost owes; the kernel's window clamp
// skipped the Sol Ring and reversed the activation (CR 601.2g, 605.3b,
// 733.1), leaving Cards[112] (the Sol Ring) untapped.
func TestCmd5OrbFeeGapSolRingFunding(t *testing.T) {
	t.Parallel()
	res := replayCmd5ReproUnverified(t, "cmd5_orb_fee_gap_sol_ring_funding", 633, "Cards[112].Flags")
	requireCmd5ReproClean(t, res, 633)
}
