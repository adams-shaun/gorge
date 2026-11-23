package oracle

import "testing"

// Magmatic Force (tail-08 pair 7, decision 485) is a may-play cast beside
// eight untapped lands under The World Tree's "{T}: Add one mana of any
// color" grant. The cast is payable (CR 605.1a, 106.6, 601.2g: eight lands,
// {5}{R}{R}{R}), but gorge's planner leaves it unpriced: the grants are a
// census gap and its prefix search (potentialScriptNodes clones) runs out
// before it reaches a paying colour assignment (census_incomplete). That is a
// gorge limit, not a lockstep divergence: the kernel's may-play gate holds its
// offer to the same budget (c52062a503, 1b2c6bca56), so both engines withhold
// the cast and agree. The test pins that agreement over the recorded game and,
// so the pin cannot go stale unseen, the planner verdict it rests on: if
// gorge's search is ever fixed to find the script, this test fails here and
// the recording becomes an offered-cast pin.
func TestMBCMagmaticForceReprosLockstep(t *testing.T) {
	t.Parallel()
	const name = "magmatic_force_world_tree_search_budget"
	r, err := ReadRepro("testdata/repro/" + name + ".repro.json")
	if err != nil {
		t.Fatal(err)
	}
	unpriced := false
	inner := &ScriptPolicy{Actions: r.Actions}
	pol := PolicyFunc(func(c *PickCtx) Canon {
		if c.Seq == 485 && c.G != nil {
			for _, pp := range c.G.E.PotentialPaymentPlans(0) {
				if pp.Action.Kind == "cast" && pp.Action.Mode == "mayplay" {
					unpriced = pp.Reason == "unsupported" && pp.Detail == "census_incomplete"
				}
			}
		}
		return inner.Pick(c)
	})
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if msg := reproSkipFailure(r, inner); msg != "" {
		t.Fatal(msg)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if !unpriced {
		t.Fatal("precondition: at decision 485 gorge's planner no longer leaves Magmatic Force's may-play cast census_incomplete; " +
			"its search now prices it, so assert the cast is offered instead")
	}
}
