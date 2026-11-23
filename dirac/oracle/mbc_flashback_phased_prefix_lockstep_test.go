package oracle

import "testing"

// CR 601.2b announces a spell's cost, then CR 601.2g-h pays it: Eviscerator's
// Insight's {4}{B} flashback sacrifices an artifact or creature as an
// additional cost, so gorge's prefix search takes its phased fallback
// (potential_script.go scriptSearch.phased) -- the two Elven Chorus granted
// taps, the four Eldrazi Spawn sacrifices, a Forest and Prismite's two {2}
// filters, in that order. The kernel's grant search now carries the same
// free/covered/fee order, so the recorded tail-51 pair 34 game replays clean
// past the payment at decision 528.
func TestMBCEvisceratorSInsightPhasedPrefixPays(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flashback_phased_prefix_pays")
}
