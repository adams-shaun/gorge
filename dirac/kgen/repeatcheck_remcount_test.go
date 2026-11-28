package kgen

import "testing"

// TestRepeatCheckRememberedIsValueCount: a RepeatCheckSVar$ over
// Remembered$Valid is a value count, as gorge evaluates the SVar (the card
// where it lies). It must not lower to cRefMatches, whose remembered read is a
// condition gate and takes a remembered card's captured last controller.
// The Tale of Tamiyo is no carrier here: its filter word is refused.
func TestRepeatCheckRememberedIsValueCount(t *testing.T) {
	cs := lowerNamed(t, []string{"Grindstone", "Sphinx's Tutelage", "Grist, the Hunger Tide"})
	seen := 0
	for _, c := range cs {
		loops := 0
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				if in.Op != "RepeatWhile" {
					continue
				}
				loops++
				cond := in.Cond
				if cond.Kind != cValue || cond.A.Kind != vRefCount || cond.A.Ref.Kind != rRemembered || cond.A.F == nil {
					t.Errorf("%s: repeat gate %v, want a value count over Remembered (%v)", c.Name, cond, c.Refusals)
				}
			}
		}
		if loops == 0 {
			t.Errorf("%s: no RepeatWhile lowered: %v", c.Name, c.Refusals)
		}
		seen++
	}
	if seen != 3 {
		t.Fatalf("lowered %d of 3 carriers", seen)
	}
}
