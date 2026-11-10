package oracle

import "testing"

// std-trc's games that once diverged at the end of their script, on the
// autopay surface: both engines now agree there, and for 200 decisions after.
func TestStdTrcReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-trc", []stdRepro{
		{"phased_out_cda_keeps_its_pt", true, false},                  // CR 604.3: a phased-out Lt. Arex's characteristic-defining P/T still sets its base, as gorge's CDASetPT reads it in every zone
		{"reg_barclay_phased_out_copy_keeps_exceptions", true, false}, // CR 702.26b: a phased-out copy keeps its layer-1 copy exceptions
		{"phased_out_copy_keeps_exceptions", true, false},             // CR 702.26b, 707.9b: Reg Barclay's token, phased out by U.S.S. Defiant, is still an Illusion artifact with haste, not the printed legendary artifact
		{"historic_saga_index_remap_animated_land", true, false},      // CR 700.6: Mutavault animated under On the Shoulders of Giants is not historic, so it gains no flying; the matchup remaps the Saga subtype PredHistoric tests
		// CR 602.2b, 106.4: Astral Cornucopia with no charge counters is a census gap gorge's grant prefix taps for nothing before Ambulatory Reactor's granted {C}
		{"zero_amount_dynamic_source_is_a_grant_gap", false, false},
	})
}
