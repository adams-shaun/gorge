package oracle

import "testing"

// TestMidPaymentUntapTapGuardLockstep replays the recorded std-msh game where
// Hydraulic Helper's {T} is paid once, Captain America, Living Legend's untap
// trigger resolves inside the payment window, and the prefix search must not
// tap the untapped Helper a second time (CR 601.2g-i through 602.2b). The
// kernel's ordered prefix taps each gap source once, so the two engines must
// agree on one tap, one Rewrite History trigger and one floating {U}.
func TestMidPaymentUntapTapGuardLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, "prefix_search_retaps_a_source_its_trigger_untapped")
}
