package oracle

import "testing"

// Kicker {B} on Warhost's Frenzy beside Signpost Scarecrow's repeatable
// "{2}: Add one mana of any color" and six untapped lands: the kicked cast is
// payable (the Scarecrow's black pays the kicker, CR 601.2f, 605.3b). Gorge's
// plain prefix search runs out of its node budget on the fee-only filter and its
// phased run finds the funding line; the kernel's search had no phased run and
// dropped the kicked option (decision 572; tail-45-c000 pair 38, seed 57367).
func TestMBCWarhostSFrenzyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicked_cast_fee_filter_phased_search")
}
