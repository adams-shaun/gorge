package cards

import "testing"

// TestManaProductionIndeterminateFlag pins the bl1 distinction between the
// two honesty flags on a source that mixes a known and an unknown amount:
//
//   - Indeterminate is set when at least one mana ability's Amount$ cannot be
//     statically priced (here the Amount$ X green ability);
//   - a KNOWN amount ability on the same source (the Amount$ 1 blue ability)
//     still contributes its guaranteed colour, so a mixed source is both
//     flagged and a real producer -- a policy that refuses all indeterminate
//     sources would wrongly drop the blue it can count on.
//
// It also pins that the indeterminate green contribution is zero, so the
// source's guaranteed-production counts (ProducesColour, DistinctColours)
// are exactly the known abilities' -- nothing claimed the pool will not get.
func TestManaProductionIndeterminateFlag(t *testing.T) {
	mp := mpOf(t, "Name:ManaBug2\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ G | Amount$ X | Oracle:x\nA:AB$ Mana | Cost$ T | Produced$ U | Amount$ 1 | Oracle:x\n")
	if !mp.Indeterminate {
		t.Error("Amount$ X ability must set Indeterminate")
	}
	if mp.Colour[1] != 1 {
		t.Errorf("known blue ability should contribute 1 blue; got %v", mp.Colour)
	}
	if mp.Colour[4] != 0 {
		t.Errorf("green from Amount$ X must not be claimed; got %v", mp.Colour)
	}
	if !mp.ProducesColour(1) || mp.ProducesColour(4) {
		t.Errorf("mixed source must produce blue but not green; got %v", mp.Colour)
	}
}
