package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// removalBoard builds the Board the A1 tests read: a castable Destroy spell
// in hand, one opposing creature, and a pass option. The spell's card facts
// are the ones both adapters fill (Castable and RemovalKind).
func removalBoard(power, toughness int32, armed bool) Board {
	b := priorityCards(map[state.ObjID]Card{
		10: {CMC: 2, Castable: true, RemovalKind: "destroy"},
	})
	b.Creatures = TableOf(map[state.ObjID]Creature{
		20: {Power: power, Toughness: toughness, Controller: 1},
	})
	b.Life = TableOf(map[state.PlayerID]int32{0: 20, 1: 20})
	if armed {
		b.Removal = RemovalRules{HoldForBigger: true}
	}
	return b
}

func removalOptions() []decision.Option {
	return []decision.Option{
		{Index: 0, Kind: "cast", Obj: 10},
		{Index: 1, Kind: "pass"},
	}
}

// TestRemovalNowHoldsSmallThreat is the A1 firing evidence: against a 3/3
// (threat 21, below the 24 bar) the armed rule holds the spell, so the same
// decision that casts it with the rule off passes instead.
func TestRemovalNowHoldsSmallThreat(t *testing.T) {
	off, _ := castDecision(removalBoard(3, 3, false), removalOptions())
	if off != 0 {
		t.Fatalf("rule off: chose option %d, want the removal cast (0)", off)
	}
	on, _ := castDecision(removalBoard(3, 3, true), removalOptions())
	if on != 1 {
		t.Fatalf("rule on: chose option %d, want the pass (1) while a 3/3 threat is below the bar", on)
	}
}

// TestRemovalNowFiresAtLargeThreat: a 6/6 (threat 36) clears the bar, so the
// rule leaves the cast alone in both configurations.
func TestRemovalNowFiresAtLargeThreat(t *testing.T) {
	for _, armed := range []bool{false, true} {
		got, d := castDecision(removalBoard(6, 6, armed), removalOptions())
		if got != 0 {
			t.Fatalf("armed=%v: chose option %d (obj %d), want the removal cast (0)", armed, got, d.Options[got].Obj)
		}
	}
}
