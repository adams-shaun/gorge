package kgen

import "testing"

// TestComponentPouchManaAbilityLowers pins that Component Pouch's first mana
// ability ({T}, remove a component counter: two mana of different colours)
// lowers beside the roll ability that puts the counters on the card. land
// reads it as an AnyDifferent gap (dirac/oracle TestComponentPouchLowers
// plays it in lockstep); this is the lowering-level guard that the card has
// no refusal left.
func TestComponentPouchManaAbilityLowers(t *testing.T) {
	const src = "Name:Pouch Probe\nManaCost:3\nTypes:Artifact\n" +
		"A:AB$ Mana | Produced$ Combo AnyDifferent | Amount$ 2 | Cost$ T SubCounter<1/COMPONENT> | SpellDescription$ Two mana, differing in color.\n" +
		"A:AB$ RollDice | Sides$ 20 | Cost$ T | ResultSubAbilities$ 1-9:PutOne,10-20:PutTwo | SpellDescription$ Roll.\n" +
		"SVar:PutOne:DB$ PutCounter | CounterType$ COMPONENT | Defined$ Self | CounterNum$ 1 | SpellDescription$ Put a counter.\n" +
		"SVar:PutTwo:DB$ PutCounter | CounterType$ COMPONENT | Defined$ Self | CounterNum$ 2 | SpellDescription$ Put two counters.\nOracle:x\n"
	_, _, rs := mbcCounters1Build(t, "Pouch Probe", src)
	if len(rs) != 0 {
		t.Fatalf("refusals = %v, want none", rs)
	}
}
