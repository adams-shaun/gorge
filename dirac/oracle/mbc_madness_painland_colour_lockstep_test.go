package oracle

import "testing"

// CR 702.35b, 601.2g: Nightshade Assassin is discarded into exile to pay
// Yawgmoth, Thran Physician's cost, and as its madness trigger resolves its
// owner holds Llanowar Wastes and Plains untapped. The madness cost {1}{B} is
// payable (the painland's {B} beside a Plains), so gorge offers the cast; the
// kernel priced it with the unless-window census, which counts a painland by
// its {C} alone, and offered only "no" (tail-15-c001 pair 6 at main
// 60022a4646, decision 742).
func TestMBCNightshadeAssassinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offer_counts_a_painland_colour")
}
