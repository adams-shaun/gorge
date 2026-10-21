package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// graveyardTargetRoundTrip casts the seat-0 hand card spell (paying mana)
// at the Grizzly Bears card in seat 0's graveyard, then, when roundTrip is
// set, moves the bears card out of the graveyard and back (a Regrowth to
// hand, then a discard) while the spell waits on the stack, and resolves
// the stack. It returns the engine, its config, the spell's id and the
// bears' id.
func graveyardTargetRoundTrip(t *testing.T, spellName, mana string, roundTrip bool) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	spell := corpusAlternativeCard(t, spellName)
	bear := corpusAlternativeCard(t, "Grizzly Bears")
	e, cfg, spellID := corpusDeckEngine(t, []*cards.Card{spell}, []*cards.Card{bear})
	var bearID state.ObjID
	for i := range e.G.Objs {
		if e.G.Objs[i].Card == bear && e.G.Objs[i].Zone == state.ZBattlefield {
			bearID = e.G.Objs[i].ID
		}
	}
	if spellID == 0 || bearID == 0 {
		t.Fatalf("setup: spell=%d bear=%d", spellID, bearID)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bearID, From: state.ZBattlefield, To: state.ZGraveyard})
	toMain1(t, e)
	addMana(t, e, 0, mana)
	submitChoices(t, e, castCardOption(t, e, spellID).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("target decision: %+v", d)
	}
	ti := indexOfObjOption(d, bearID)
	if ti < 0 {
		t.Fatalf("target ask does not offer the graveyard bears: %+v", d.Options)
	}
	submitChoices(t, e, ti)
	if e.G.Obj(spellID).Zone != state.ZStack {
		t.Fatalf("precondition: %s must be on the stack, zone %v", spellName, e.G.Obj(spellID).Zone)
	}
	if roundTrip {
		// Regrowth returns the bears card to hand, then it is discarded: it
		// never touches the battlefield, so only CR 400.7 makes it new.
		e.emit(events.Event{Kind: events.MoveZone, Obj: bearID, From: state.ZGraveyard, To: state.ZHand})
		e.emit(events.Event{Kind: events.MoveZone, Obj: bearID, From: state.ZHand, To: state.ZGraveyard})
		if e.G.Obj(bearID).Zone != state.ZGraveyard {
			t.Fatal("precondition: the bears card must be back in the graveyard")
		}
	}
	passUntilStackEmpty(t, e, 20)
	return e, cfg, spellID, bearID
}

// TestCR400_7GraveyardTargetThatLeftAndReturnedIsANewObject pins CR 400.7 /
// 608.2b off the battlefield: Raise Dead targets a creature card in the
// graveyard; the card goes to hand and is discarded back before Raise Dead
// resolves. The card in the graveyard is a new object, so Raise Dead has no
// legal target and does nothing. The control arm (no round trip) returns
// the card.
func TestCR400_7GraveyardTargetThatLeftAndReturnedIsANewObject(t *testing.T) {
	t.Parallel()
	for _, roundTrip := range []bool{false, true} {
		e, cfg, spellID, bearID := graveyardTargetRoundTrip(t, "Raise Dead", "B", roundTrip)
		want := state.ZHand
		if roundTrip {
			want = state.ZGraveyard
		}
		if z := e.G.Obj(bearID).Zone; z != want {
			t.Fatalf("roundTrip=%v: bears zone %v, want %v", roundTrip, z, want)
		}
		if z := e.G.Obj(spellID).Zone; z != state.ZGraveyard {
			t.Fatalf("roundTrip=%v: Raise Dead zone %v, want graveyard", roundTrip, z)
		}
		replayCheck(t, e, cfg)
	}
}

// TestCR400_7AnimateDeadTargetThatLeftAndReturnedIsANewObject is the audit's
// Animate Dead repro (S05 #11): Animate Dead targets a creature card in a
// graveyard, the card is Regrowth'd and discarded again in response, and
// Animate Dead resolves. Its target is a new object, so the Aura spell has
// no legal target, does not resolve and reanimates nothing (CR 608.2b,
// 303.4d). The control arm reanimates the bears.
func TestCR400_7AnimateDeadTargetThatLeftAndReturnedIsANewObject(t *testing.T) {
	t.Parallel()
	for _, roundTrip := range []bool{false, true} {
		e, cfg, spellID, bearID := graveyardTargetRoundTrip(t, "Animate Dead", "1B", roundTrip)
		wantBear, wantAura := state.ZBattlefield, state.ZBattlefield
		if roundTrip {
			wantBear, wantAura = state.ZGraveyard, state.ZGraveyard
		}
		if z := e.G.Obj(bearID).Zone; z != wantBear {
			t.Fatalf("roundTrip=%v: bears zone %v, want %v", roundTrip, z, wantBear)
		}
		if z := e.G.Obj(spellID).Zone; z != wantAura {
			t.Fatalf("roundTrip=%v: Animate Dead zone %v, want %v", roundTrip, z, wantAura)
		}
		replayCheck(t, e, cfg)
	}
}
