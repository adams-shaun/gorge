package oracle

import "testing"

// A restricted {C} source beneath a Taps watcher funds a non-spell payment
// (CR 106.6, 601.2g, 602.5d): a Powerstone token under Relic Putrescence is
// the seventh source of A-Thran Spider's {7} activation, which gorge pays by
// tapping the token first and stacking the watcher's poison trigger. The
// kernel's Taps-deferred restricted gap admitted one fixed colour only, so
// the activation stalled (restrdefer, decision 514 of tail-13-c000 pair 44).
func TestMBCTail13ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restr_taps_gap_colourless_source")
}
