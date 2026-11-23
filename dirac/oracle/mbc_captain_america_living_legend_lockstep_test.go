package oracle

import "testing"

// Paying an activation through a mana prefix whose tap triggers an untap-me
// trigger (Captain America, Living Legend), gorge's prefix search resolves the
// trigger the sorcery-speed play must wait out where it was stacked before it
// branches further taps under it (CR 605.3b, 603.3, 602.5d), as the kernel's
// filtSearch.settle does, so one tap makes one trigger and one {U} (std-msh
// focus pair 182, decision 776). The card was refused by name while the two
// walks disagreed; the recorded game now replays clean.
func TestMBCCaptainAmericaLivingLegendReprosLockstep(t *testing.T) {
	t.Parallel()
	res := replayRawRepro(t, "prefix_search_retaps_a_source_its_trigger_untapped")
	if res.Div != nil || res.StallUnmodelled {
		t.Fatalf("the recorded game does not replay clean: %+v", res.Div)
	}
}
