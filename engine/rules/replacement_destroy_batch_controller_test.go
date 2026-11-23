package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDestroyLineOfDepartedBatchMemberReadsItsBattlefieldController: the medic
// (owned by seat 1, controlled by seat 0) dies to lethal damage in the same SBA
// batch as two bears carrying +1/+1 counters. Its "creature you control" line
// still applies to the batch (CR 614.6) for the creature seat 0 controls and
// never for seat 1's: the medic's live controller after it left is its owner,
// seat 1, which must not leak into the line's YouCtrl.
func TestDestroyLineOfDepartedBatchMemberReadsItsBattlefieldController(t *testing.T) {
	medicCard := card(t, destroyLineMedicSrc)
	mine := card(t, "Name:Mine Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	theirs := card(t, destroyLineBearSrc)
	e, _ := tokenReplGameSeats(t, 13, []*cards.Card{mine}, []*cards.Card{medicCard, theirs})
	medic := moveSeededCard(t, e, 1, medicCard, state.ZBattlefield)
	e.G.Obj(medic).Controller = 0
	myBear := moveSeededCard(t, e, 0, mine, state.ZBattlefield)
	oppBear := moveSeededCard(t, e, 1, theirs, state.ZBattlefield)
	for _, id := range []state.ObjID{medic, myBear, oppBear} {
		if id == medic {
			e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 2})
			continue
		}
		e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1})
		e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 3})
	}
	e.checkStateBased()
	if z := e.G.Obj(medic).Zone; z != state.ZGraveyard {
		t.Fatalf("medic zone %s, want graveyard (precondition)", z)
	}
	if z := e.G.Obj(myBear).Zone; z != state.ZBattlefield {
		t.Errorf("the creature the medic's controller controls: zone %s, want spared", z)
	}
	if z := e.G.Obj(oppBear).Zone; z != state.ZGraveyard {
		t.Errorf("the creature the medic's controller does not control: zone %s, want destroyed", z)
	}
}
