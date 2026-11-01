package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTargetThatDiedAndReturnedIsANewObject pins CR 400.7 / 608.2b: a
// permanent that leaves the battlefield and returns is a new object, so a
// spell that targeted it before it left has no legal target when it
// resolves, even though gorge's ObjID survives the round trip. The stack
// object stamps the target's Incarnation when it is chosen
// (state.Object.TargetStamps) and the resolution recheck drops a target
// whose Incarnation has moved. dirac pairs 1699 3455 (Fleeting
// Distraction / Stab on a returned Infernal Vessel / Infestation Sage) and
// 14299 46767 66849 (Axgard Cavalry's haste, Sower of Chaos's can't-block
// on a returned Reassembling Skeleton / Infernal Vessel). The control arm
// resolves the same spell with no round trip and must pump.
func TestTargetThatDiedAndReturnedIsANewObject(t *testing.T) {
	t.Parallel()
	for _, roundTrip := range []bool{false, true} {
		growth := card(t, "Name:Test Growth\nManaCost:R\nTypes:Instant\n"+
			"A:SP$ Pump | ValidTgts$ Creature | NumAtt$ +3 | NumDef$ +3 | SpellDescription$ x\nOracle:x\n")
		bear := card(t, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		e, cfg, growthID := corpusDeckEngine(t, []*cards.Card{growth}, []*cards.Card{bear})
		var bearID state.ObjID
		for i := range e.G.Objs {
			if e.G.Objs[i].Card == bear && e.G.Objs[i].Zone == state.ZBattlefield {
				bearID = e.G.Objs[i].ID
			}
		}
		if growthID == 0 || bearID == 0 {
			t.Fatalf("setup: growth=%d bear=%d", growthID, bearID)
		}
		toMain1(t, e)
		addMana(t, e, 0, "R")
		submitChoices(t, e, castCardOption(t, e, growthID).Index)
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("target decision: %+v", d)
		}
		ti := indexOfObjOption(d, bearID)
		if ti < 0 {
			t.Fatalf("target ask does not offer the bear: %+v", d.Options)
		}
		submitChoices(t, e, ti)
		if e.G.Obj(growthID).Zone != state.ZStack {
			t.Fatalf("precondition: Test Growth must be on the stack, zone %v", e.G.Obj(growthID).Zone)
		}
		if roundTrip {
			// The bear dies and comes straight back (its own dies trigger, in
			// the reported games) while the spell waits on the stack.
			e.emit(events.Event{Kind: events.MoveZone, Obj: bearID, From: state.ZBattlefield, To: state.ZGraveyard})
			e.emit(events.Event{Kind: events.MoveZone, Obj: bearID, From: state.ZGraveyard, To: state.ZBattlefield})
			if e.G.Obj(bearID).Zone != state.ZBattlefield {
				t.Fatal("precondition: the bear must be back on the battlefield")
			}
		}
		passUntilStackEmpty(t, e, 20)
		want := int32(5)
		if roundTrip {
			want = 2
		}
		if got := e.Power(bearID); got != want {
			t.Fatalf("roundTrip=%v: bear power = %d, want %d", roundTrip, got, want)
		}
		if z := e.G.Obj(growthID).Zone; z != state.ZGraveyard {
			t.Fatalf("roundTrip=%v: Test Growth zone %v, want graveyard", roundTrip, z)
		}
		replayCheck(t, e, cfg)
	}
}
