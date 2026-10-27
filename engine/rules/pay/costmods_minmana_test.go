package pay

import "testing"

// TestMinManaFloorCapsZeroManaCost pins the MinMana$ floor as a CAP on one
// reduction, not a minimum price: Forge CostAdjustment.java:500-509 computes
// maxReduction = manaCost.cmc - minMana - sumReduced and returns 0 when that
// is not positive, so a reduction never ADDS mana to a cost already below the
// floor. A zero-mana Cost with a {1} reduction floored at 1 stays zero.
func TestMinManaFloorCapsZeroManaCost(t *testing.T) {
	t.Parallel()
	c := Cost{}
	if got := c.CMC(); got != 0 {
		t.Fatalf("precondition: the fixture cost has %d mana, want 0", got)
	}
	m := CostMods{Reduces: []CostMod{{Generic: 1, Floor: 1}}}
	m.ApplyTo(&c)
	if c.Generic != 0 || c.CMC() != 0 {
		t.Fatalf("a MinMana$ 1 floor raised a zero-mana cost to generic %d, want it left at zero", c.Generic)
	}
}
