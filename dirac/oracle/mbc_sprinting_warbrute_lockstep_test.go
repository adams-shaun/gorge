package oracle

import "testing"

const worldTreeRedPrefixBudgetRepro = "world_tree_any_colour_red_prefix_budget"

// CR 106.1, 605.1a, 601.2g: once a Rain of Filth turn has made every other
// land sacrifice for {B}, The World Tree's "lands you control have '{T}: Add
// one mana of any color'" is the only red p0 can reach, so Sprinting Warbrute
// ({4}{R}, dash {3}{R}) is payable by tapping a land for {R} and sacrificing
// lands for the rest. The recorded game (tail-08 pair 62, decision 406) once
// showed the kernel offer both casts while gorge's potentialPrefixScript spent
// its 160-clone budget on the sacrifice answers and withheld them. The filter
// prefix search now counts gorge's free {C} node and counter-ability colour
// ask in its budget (aad8a3ce95), so both engines agree on the offer; the card
// is not refused and this test pins the recorded game replaying clean.
func TestMBCSprintingWarbruteReprosLockstep(t *testing.T) {
	res := replayRawRepro(t, worldTreeRedPrefixBudgetRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", worldTreeRedPrefixBudgetRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", worldTreeRedPrefixBudgetRepro, res.Stall)
	}
}
