package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// trimHandTo moves seat 0's hand cards other than keepOut back to the library
// until exactly n remain, and returns them in zone order.
func trimHandTo(t *testing.T, e *Engine, keepOut state.ObjID, n int) []state.ObjID {
	t.Helper()
	var others []state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if id != keepOut {
			others = append(others, id)
		}
	}
	for len(others) > n {
		id := others[len(others)-1]
		others = others[:len(others)-1]
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	if len(others) != n {
		t.Fatalf("hand holds %d other cards, want %d", len(others), n)
	}
	return others
}

const cloakFromHandSrc = "Name:Hand Cloak\nManaCost:0\nTypes:Instant\nA:SP$ Cloak | Choices$ Card.YouCtrl\nOracle:x\n"

func TestCloakFromHandAsksAndCloaksTheChosenCard(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 4411, cloakFromHandSrc)
	others := trimHandTo(t, e, id, 2)
	if others[0] == others[1] {
		t.Fatal("precondition: two distinct hand cards")
	}
	e.pending = nil
	e.Advance()
	castFromPriority(t, e, id)
	d := passUntilAskKind(t, e, decision.KChoose, 20)
	if d.Min != 1 || d.Max != 1 || len(d.Options) != 2 ||
		d.Options[0].Obj != others[0] || d.Options[1].Obj != others[1] {
		t.Fatalf("ask = %+v, want min=max=1 over %v", d, others)
	}
	submitChoices(t, e, 1)
	passUntilStackEmpty(t, e, 20)
	got := cloakedOn(t, e, 0)
	if got != others[1] {
		t.Fatalf("cloaked %d, want the second offered card %d", got, others[1])
	}
	if e.G.Obj(others[0]).Zone != state.ZHand {
		t.Fatalf("the unchosen card left the hand: zone %v", e.G.Obj(others[0]).Zone)
	}
	noUnimplementedCloak(t, e)
	replayCheck(t, e, cfg)
}

func TestCloakFromEmptyHandDoesNothing(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 4412, cloakFromHandSrc)
	trimHandTo(t, e, id, 0)
	e.pending = nil
	e.Advance()
	castFromPriority(t, e, id)
	passUntilStackEmpty(t, e, 20)
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.To == state.ZBattlefield && ev.Counter == events.CloakEntryCounter {
			t.Fatal("an empty hand cloaked a card")
		}
	}
	noUnimplementedCloak(t, e)
	replayCheck(t, e, cfg)
}

func TestVannifarCharmCloaksFromHand(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Vannifar, Evolved Enigma", "Grizzly Bears")
	van := searchMoveByName(t, e, "Vannifar, Evolved Enigma", state.ZHand)
	e.emit(events.Event{Kind: events.MoveZone, Obj: van, From: state.ZHand, To: state.ZBattlefield})
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	trimHandTo(t, e, 0, 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZLibrary, To: state.ZHand})
	if e.G.Obj(bear).Zone != state.ZHand {
		t.Fatal("precondition: the bear is in hand")
	}
	e.pending = nil
	e.Advance()
	d := passUntilAskKind(t, e, decision.KModes, 20)
	submitChoices(t, e, 0)
	d = passUntilAskKind(t, e, decision.KChoose, 20)
	if len(d.Options) != 1 || d.Options[0].Obj != bear {
		t.Fatalf("hand ask = %+v, want only the bear", d.Options)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if got := cloakedOn(t, e, 0); got != bear {
		t.Fatalf("cloaked %d, want the bear %d", got, bear)
	}
	noUnimplementedCloak(t, e)
	replayCheck(t, e, cfg)
}

func TestScrollOfFateManifestsAChosenHandCard(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Scroll of Fate", "Grizzly Bears")
	scroll := searchMoveByName(t, e, "Scroll of Fate", state.ZHand)
	e.emit(events.Event{Kind: events.MoveZone, Obj: scroll, From: state.ZHand, To: state.ZBattlefield})
	others := trimHandTo(t, e, 0, 2)
	e.pending = nil
	e.Advance()
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == scroll {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no activation offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = passUntilAskKind(t, e, decision.KChoose, 20)
	if len(d.Options) != 2 || d.Options[1].Obj != others[1] {
		t.Fatalf("ask = %+v, want %v", d.Options, others)
	}
	submitChoices(t, e, 1)
	passUntilStackEmpty(t, e, 20)
	if got := manifestedOn(t, e, 0); got != others[1] {
		t.Fatalf("manifested %d, want %d", got, others[1])
	}
	if e.G.Obj(others[1]).Cloaked {
		t.Fatal("a manifest was marked cloaked")
	}
	noUnimplementedManifest(t, e)
	replayCheck(t, e, cfg)
}
