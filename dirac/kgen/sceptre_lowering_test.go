package kgen

import "testing"

// TestSceptreLowering: Sceptre of Eternal Glory lowers with both mana
// abilities; the second ({T}: three mana of any one colour, "activate only if
// you control three or more lands with the same name") carries its CheckSVar$
// gate as the mana ability's ActCond (built into Trig.Cond) (the planner/window variant of
// dirac/mana_gate.go), the first stays ungated.
func TestSceptreLowering(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Sceptre of Eternal Glory"})
	if len(lw.cards) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(lw.cards))
	}
	c := lw.cards[0]
	if len(c.Refusals) != 0 {
		t.Fatalf("%s refused: %s", c.Name, abReasons(c))
	}
	mana := 0
	gated := 0
	for _, ab := range c.Abilities {
		if ab.Kind != akMana {
			continue
		}
		mana++
		if ab.ActCond.Kind != cNone {
			gated++
		}
	}
	if mana != 2 || gated != 1 {
		t.Errorf("mana abilities %d (gated %d), want 2 with 1 gated", mana, gated)
	}
}
