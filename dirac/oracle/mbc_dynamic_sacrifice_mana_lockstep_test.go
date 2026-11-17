package oracle

import "testing"

const dynamicSacrificeManaRepro = "dynamic_sacrifice_mana_gap_unmodelled"

// CR 601.2g with CR 605.1a: Shrine of Boundless Growth's "{T}, sacrifice this:
// add {C} for each charge counter" is a mana ability its controller may
// activate while paying. gorge's planner census cannot price the evaluated
// amount beside the sacrifice cost, so its potential walk activates the Shrine
// and offers Bushmeat Poacher's {3}{B} a prefix through it pays (two Plains, a
// Swamp and a charged Shrine; decision 140, tail-34 pair 0). The kernel lists
// the counter-scaled source among its counter-cost script gaps
// (mana_sacctr.go), so the pinned cast is offered too.
func TestMBCBushmeatPoacherReprosLockstep(t *testing.T) {
	t.Parallel()
	if res := replayRawRepro(t, dynamicSacrificeManaRepro); res.Div != nil || res.StallUnmodelled {
		t.Fatalf("divergence at %d: %s %s %v (stall %q)", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall)
	}
}
