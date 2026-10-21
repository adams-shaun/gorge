package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR400_7ValkyriesCallReturnsOnlyTheCardThatDied pins CR 400.7 / 400.7e
// for a trigger's "that card": Grizzly Bears die under Valkyrie's Call, and
// while the return trigger waits, the bears card leaves the graveyard and
// comes back (Regrowth, then a discard). The card in the graveyard is a new
// object the trigger cannot find, so it stays there. The control arm (no
// round trip) returns the bears to the battlefield.
func TestCR400_7ValkyriesCallReturnsOnlyTheCardThatDied(t *testing.T) {
	t.Parallel()
	for _, roundTrip := range []bool{false, true} {
		valk := corpusAlternativeCard(t, "Valkyrie's Call")
		bear := corpusAlternativeCard(t, "Grizzly Bears")
		e, cfg, _ := corpusDeckEngine(t, nil, []*cards.Card{valk, bear})
		var bearID state.ObjID
		for i := range e.G.Objs {
			if e.G.Objs[i].Card == bear && e.G.Objs[i].Zone == state.ZBattlefield {
				bearID = e.G.Objs[i].ID
			}
		}
		if bearID == 0 {
			t.Fatal("setup: no bears on the battlefield")
		}
		toMain1(t, e)
		e.emit(events.Event{Kind: events.MoveZone, Obj: bearID, From: state.ZBattlefield, To: state.ZGraveyard})
		if roundTrip {
			e.emit(events.Event{Kind: events.MoveZone, Obj: bearID, From: state.ZGraveyard, To: state.ZHand})
			e.emit(events.Event{Kind: events.MoveZone, Obj: bearID, From: state.ZHand, To: state.ZGraveyard})
		}
		e.pending = nil
		e.Advance()
		passUntilStackEmpty(t, e, 20)
		want := state.ZBattlefield
		if roundTrip {
			want = state.ZGraveyard
		}
		if z := e.G.Obj(bearID).Zone; z != want {
			t.Fatalf("roundTrip=%v: bears zone %v, want %v", roundTrip, z, want)
		}
		replayCheck(t, e, cfg)
	}
}

// TestCR400_7GraveyardGrantEndsWhenTheCardChangesZones pins CR 400.7 for a
// grant to a card off the battlefield: Snapcaster Mage's "target instant or
// sorcery card in your graveyard gains flashback until end of turn" (the
// corpus line's shape). The card goes to hand and is discarded again the
// same turn: it is a new object without flashback. The control arm keeps it.
func TestCR400_7GraveyardGrantEndsWhenTheCardChangesZones(t *testing.T) {
	t.Parallel()
	for _, roundTrip := range []bool{false, true} {
		snap := "Name:Snapcaster\nManaCost:1 U\nTypes:Creature Human Wizard\nPT:2/1\n" +
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigFlashback\n" +
			"SVar:TrigFlashback:DB$ Pump | ValidTgts$ Instant.YouCtrl,Sorcery.YouCtrl | TgtZone$ Graveyard | TgtPrompt$ Select target instant or sorcery card | KW$ Flashback | PumpZone$ Graveyard\nOracle:x\n"
		inst := "Name:Pyro\nManaCost:1 R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"
		e, cfg, _ := newFixtureDeck(t, 6203, snap, inst)
		addToGraveyard(t, e, 0, inst)
		putCreature(t, e, 0, snap)
		e.Advance()
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget || d.Options[0].Obj == 0 {
			t.Fatalf("pending = %+v, want the ETB target ask over the graveyard instant", d)
		}
		submitChoices(t, e, 0)
		passUntilStackEmpty(t, e, 20)
		pyro := findByName(e, "Pyro", 0)
		if !e.HasKeyword(pyro, "Flashback") {
			t.Fatalf("precondition: the graveyard card has no flashback: %v", e.Derived(pyro).Keywords)
		}
		if roundTrip {
			e.emit(events.Event{Kind: events.MoveZone, Obj: pyro, From: state.ZGraveyard, To: state.ZHand})
			e.emit(events.Event{Kind: events.MoveZone, Obj: pyro, From: state.ZHand, To: state.ZGraveyard})
		}
		if got := e.HasKeyword(pyro, "Flashback"); got == roundTrip {
			t.Fatalf("roundTrip=%v: graveyard card has flashback=%v, want %v", roundTrip, got, !roundTrip)
		}
		replayCheck(t, e, cfg)
	}
}
