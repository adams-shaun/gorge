package kgen

import "testing"

// TestManaRiderSelfGainLife: a tap-only mana ability whose rider is "you gain
// N life" (Pristine Talisman, The Great Henge) lowers as the bounded
// self-life rider (CR 605.3b), carrying its constant N into ManaRiderN.
func TestManaRiderSelfGainLife(t *testing.T) {
	want := map[string]int{"Pristine Talisman": 1, "The Great Henge": 2}
	names := []string{"Pristine Talisman", "The Great Henge"}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		n, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
			continue
		}
		found := false
		for _, a := range c.Abilities {
			if a.Kind == akMana && a.Mana != nil && a.Mana.Rider == riderSelfGainLife && a.Mana.RiderN == n {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no mana ability with the self-life rider, N %d", c.Name, n)
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d cards", seen, len(want))
	}
}
