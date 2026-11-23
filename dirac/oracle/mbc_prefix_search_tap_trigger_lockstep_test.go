package oracle

import "testing"

// CR 605.3b, 603.3, 602.5d: with Corrupted Roots on one of four Forests and
// one life left, Hall of Tagsin's "{1},{T}: any color" funded by another Forest
// makes the third {B} of Underworld Dreams. Tapping the enchanted Forest stacks
// a lethal trigger the sorcery-speed cast must wait out, and the pass that
// resolves it kills the payer. Gorge's prefix search counts that dead pass
// child a node under every funding tap (two nodes a funder, 160 spent before
// the cast is priced: census_incomplete), where the kernel's filter search
// charged the lethal pass nothing and stopped at 134, offering the cast
// (decision 724 of tail-39-c000 pair 6). The kernel now charges the dead pass
// its tick (filtSearch.passTrigger), so both walks run out of budget together.
func TestMBCUnderworldDreamsReprosLockstep(t *testing.T) {
	t.Parallel()
	res := replayRawRepro(t, "prefix_search_lethal_tap_trigger_budget")
	if res.Div != nil || res.StallUnmodelled {
		t.Fatalf("the recorded game does not replay clean: %+v", res.Div)
	}
}
