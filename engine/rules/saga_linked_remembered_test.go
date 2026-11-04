package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMeasureOfAManChapterIIIReturnsChapterIExile drives The Measure of a
// Man's three chapters through the engine. Chapters I and III are linked
// abilities (CR 607.2a, 714.2b): chapter III's "the exiled card" is the card
// chapter I exiled, which the Saga remembers on its own persistent list. A
// chapter trigger starts with an empty remembered set and no capture, so its
// Defined$ Remembered read must fall back to that list; before the fix it
// read nothing and the creature stayed in exile.
func TestMeasureOfAManChapterIIIReturnsChapterIExile(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	measure := mustCorpusCard(t, reg, "The Measure of a Man")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	cfg := Config{Seed: 607, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{measure}, mountainDeck(t, 39)...),
			append([]*cards.Card{bear}, mountainDeck(t, 39)...),
		}}
	e := New(seatZeroStart(cfg))
	e.Advance()

	victim := placeInDeck(t, e, 1, bear, state.ZBattlefield)
	saga := placeInDeck(t, e, 0, measure, state.ZBattlefield)
	if got := e.G.Obj(saga).Counter("LORE"); got != 1 {
		t.Fatalf("precondition: lore = %d after entry, want 1", got)
	}
	drainMeasure(t, e, victim)
	if z := e.G.Obj(victim).Zone; z != state.ZExile {
		t.Fatalf("chapter I: Grizzly Bears zone = %s, want Exile", z)
	}

	life := e.G.Players[0].Life
	for chapter := 2; chapter <= 3; chapter++ {
		e.SetCounterAdder(0)
		e.emit(events.Event{Kind: events.CounterChange, Obj: saga, Counter: "LORE", Amount: 1})
		drainMeasure(t, e, victim)
	}
	if o := e.G.Obj(victim); o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("chapter III: Grizzly Bears zone = %s controller %d, want Battlefield under its owner (seat 1)", o.Zone, o.Controller)
	}
	if got := e.G.Players[0].Life; got != life+2 {
		t.Fatalf("chapter III: life = %d, want %d", got, life+2)
	}
	replayCheck(t, e, cfg)
}

// drainMeasure puts the queued chapter on the stack and resolves it,
// targeting victim when a target is asked.
func drainMeasure(t *testing.T, e *Engine, victim state.ObjID) {
	t.Helper()
	e.putTriggersOnStack()
	for i := 0; i < 100 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			d = e.Pending()
		}
		if d == nil || (d.Kind == decision.KPriority && len(e.G.Stack) == 0) {
			return
		}
		switch d.Kind {
		case decision.KPriority:
			submitPass(t, e)
		case decision.KTarget:
			answerKTarget(t, e, victim)
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
	}
}

// TestBestOfBothWorldsChapterIIIReturnsTheAssimilated drives The Best of Both
// Worlds: chapter I assimilates an opponent's creature and remembers it on the
// Saga, and chapter III, linked to it (CR 607.2a), exiles that creature and
// returns it under its owner's control as a new object (CR 400.7): no
// counter, no longer an artifact, back with seat 1.
func TestBestOfBothWorldsChapterIIIReturnsTheAssimilated(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	saga := mustCorpusCard(t, reg, "The Best of Both Worlds")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	cfg := Config{Seed: 714, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{saga}, mountainDeck(t, 39)...),
			append([]*cards.Card{bear}, mountainDeck(t, 39)...),
		}}
	e := New(seatZeroStart(cfg))
	e.Advance()

	victim := placeInDeck(t, e, 1, bear, state.ZBattlefield)
	sid := placeInDeck(t, e, 0, saga, state.ZBattlefield)
	drainMeasure(t, e, victim)
	if o := e.G.Obj(victim); o.Zone != state.ZBattlefield || o.Controller != 0 || o.Counter("P1P1") != 1 {
		t.Fatalf("chapter I: Grizzly Bears zone %s controller %d +1/+1 %d, want assimilated by seat 0",
			o.Zone, o.Controller, o.Counter("P1P1"))
	}
	for chapter := 2; chapter <= 3; chapter++ {
		e.SetCounterAdder(0)
		e.emit(events.Event{Kind: events.CounterChange, Obj: sid, Counter: "LORE", Amount: 1})
		drainMeasure(t, e, victim)
	}
	if o := e.G.Obj(victim); o.Zone != state.ZBattlefield || o.Controller != 1 || o.Counter("P1P1") != 0 {
		t.Fatalf("chapter III: Grizzly Bears zone %s controller %d +1/+1 %d, want returned to seat 1 with no counter",
			o.Zone, o.Controller, o.Counter("P1P1"))
	}
	replayCheck(t, e, cfg)
}
