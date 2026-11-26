package kgen

import (
	"testing"
)

// TestKiliAttachAlternative: Kíli the Resourceful's static
// (S:Mode$ AlternativeCost | ValidSA$ Activated.Equip | ValidPlayer$ You |
// Cost$ 0 | Condition$ EnduringStory | CheckSVar$ X | SVarCompare$ EQ0,
// gorge activate_static_altcost.go's attach-cost alternative grant) lowers
// whole: no refusals, one AltEquipCost static whose gates are the enduring
// story beside the zero count of the turn's first equip (X =
// Count$ThisTurnActivated_Activated.Equip+YouCtrl, EQ0; the count gate reads
// 0 only before the turn's first equip push), whose filter is the no
// ValidCard$ any-permanent one, whose ValidPlayer$ You keeps the activator
// the static's controller (no AnyCaster flag) and whose Cost$ 0 lowers to
// the empty mana cost.
func TestKiliAttachAlternative(t *testing.T) {
	c := lowerCorpusCard(t, "Kíli the Resourceful")
	if len(c.Refusals) != 0 {
		t.Fatalf("Kíli the Resourceful refusals %v, want none", c.Refusals)
	}
	n := 0
	for _, st := range c.Statics {
		if st.Kind != "AltEquipCost" {
			continue
		}
		n++
		// the gates: Condition$ EnduringStory beside CheckSVar$ X,
		// SVarCompare$ EQ0 (the count gate is the composite's second clause)
		if st.Cond.Kind != cAnd || len(st.Cond.Sub) != 2 {
			t.Errorf("AltEquipCost cond = %v, want EnduringStory and the count gate", st.Cond)
		} else {
			if st.Cond.Sub[0].Kind != cEnduringStory {
				t.Errorf("AltEquipCost cond[0] = %v, want EnduringStory", st.Cond.Sub[0])
			}
			cc := st.Cond.Sub[1]
			if cc.Kind != cValue || cc.A.Kind != vThisTurnActExhaust || cc.A.N != 1 || cc.Op != opEQ || cc.B.Kind != vConst || cc.B.N != 0 {
				t.Errorf("AltEquipCost cond[1] = %v, want ThisTurnActExhaust N 1 EQ 0", cc)
			}
		}
		if st.F == nil {
			t.Error("AltEquipCost: no ValidCard$ lowers to the any-permanent filter, got none")
		}
		if !st.Cost.pureMana() || st.Cost.Mana.Generic != 0 || st.Cost.Mana.Pips != [5]int{} ||
			st.Cost.Mana.C != 0 || st.Cost.Mana.Snow != 0 || st.Cost.Mana.X {
			t.Errorf("AltEquipCost cost = %+v, want the empty {0}", st.Cost)
		}
		if len(st.Flags) != 0 {
			t.Errorf("AltEquipCost flags = %v, want none (ValidPlayer$ You present, no AnyCaster)", st.Flags)
		}
	}
	if n != 1 {
		t.Errorf("Kíli the Resourceful lowers to %d AltEquipCost statics, want 1", n)
	}
}
