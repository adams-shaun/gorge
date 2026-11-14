package kgen

import (
	"strings"
	"testing"
)

// TestComponentPouchManaAbilityStaysRefused pins why Component Pouch's first
// mana ability ({T}, remove a component counter: two mana of different
// colours) is refused: its cost is T+RemoveCtr and the kernel's mana walk
// cannot activate it as a first ability. The COMPONENT counter kind itself
// lowers (the roll ability puts it on the card), so the mana ability is the
// card's only refusal. gorge cannot resolve that production at all (its mana
// effect rejects the Combo AnyDifferent token and poses no colour ask), so
// there are no colour answers to mirror; a follow-up lowers it once gorge
// resolves the production. A lowering of this shape that lands without that
// fails here, so the lowering has to be a deliberate, reviewed change.
func TestComponentPouchManaAbilityStaysRefused(t *testing.T) {
	const src = "Name:Pouch Probe\nManaCost:3\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T SubCounter<1/COMPONENT> | Amount$ 2 | Produced$ Combo AnyDifferent | SpellDescription$ Add two mana of different colors.\n" +
		"A:AB$ RollDice | Cost$ T | Sides$ 20 | ResultSubAbilities$ 1-9:PutOne,10-20:PutTwo | SpellDescription$ Roll a d20.\n" +
		"SVar:PutOne:DB$ PutCounter | Defined$ Self | CounterType$ COMPONENT | CounterNum$ 1 | SpellDescription$ Put a counter.\n" +
		"SVar:PutTwo:DB$ PutCounter | Defined$ Self | CounterType$ COMPONENT | CounterNum$ 2 | SpellDescription$ Put two counters.\nOracle:x\n"
	_, _, rs := mbcCounters1Build(t, "Pouch Probe", src)
	if len(rs) != 1 {
		t.Fatalf("refusals = %v, want exactly the mana ability's", rs)
	}
	if !strings.Contains(rs[0].Node, "mana ability") || !strings.Contains(rs[0].Reason, "mana ability cost T+RemoveCtr") {
		t.Fatalf("refusal = %+v, want the emit-mana-ability T+RemoveCtr reason", rs[0])
	}
}
