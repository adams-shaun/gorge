package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// geistsHold sets up Dungeon Geists entering under seat 0 against seat 1's
// tapped Bear, and returns the engine with the Geists and the Bear ids once the
// enters trigger has resolved.
func geistsHold(t *testing.T) (e *Engine, geists, bear state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	dg, ok := reg.Lookup("Dungeon Geists")
	if !ok {
		t.Fatalf("corpus fixture: Dungeon Geists missing")
	}
	cfg := seatZeroStart(Config{Seed: 4243, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{dg}, mountainDeck(t, 39)...),
			append([]*cards.Card{card(t, udBear)}, mountainDeck(t, 39)...),
		},
		Tokens: map[string]*cards.Card{},
	})
	e = New(cfg)
	e.Advance()
	bear = moveByName(t, e, 1, "Ud Bear", state.ZBattlefield)
	geists = moveByName(t, e, 0, "Dungeon Geists", state.ZHand)
	addMana(t, e, 0, "UUUU")
	submitChoices(t, e, castOptionFor(t, e, geists).Index)
	if d := passPriorityUntilNonPriority(t, e); d.Kind == decision.KTarget {
		pick := -1
		for _, o := range d.Options {
			if o.Obj == bear {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("the Bear was not offered as a target: %+v", d.Options)
		}
		submitChoices(t, e, pick)
	}
	passUntilStackEmpty(t, e, 40)
	if !e.G.Obj(bear).Tapped {
		t.Fatalf("precondition: Geists' enters trigger did not tap the Bear")
	}
	return e, geists, bear
}

// TestAsLongAsControlEndsOnControlChange pins CR 611.2b: Dungeon Geists'
// "doesn't untap for as long as you control this creature" ends when its
// controller stops controlling it, not only when it leaves the battlefield.
func TestAsLongAsControlEndsOnControlChange(t *testing.T) {
	t.Parallel()
	t.Run("still controlled holds the creature", func(t *testing.T) {
		t.Parallel()
		e, geists, bear := geistsHold(t)
		driveToStepAny(t, e, 2, 1, state.StepUpkeep)
		if e.G.Obj(geists).Controller != 0 {
			t.Fatalf("precondition: Geists controller = %d, want 0", e.G.Obj(geists).Controller)
		}
		if !e.G.Obj(bear).Tapped {
			t.Fatalf("the Bear untapped while its holder's controller still controls Geists")
		}
	})
	t.Run("stolen Geists releases the creature", func(t *testing.T) {
		t.Parallel()
		e, geists, bear := geistsHold(t)
		e.emit(events.Event{Kind: events.ControlChange, Obj: geists, Player: 1})
		if e.G.Obj(geists).Controller != 1 {
			t.Fatalf("precondition: Geists controller = %d, want 1", e.G.Obj(geists).Controller)
		}
		driveToStepAny(t, e, 2, 1, state.StepUpkeep)
		if e.G.Obj(bear).Tapped {
			t.Fatalf("the Bear stayed tapped after its holder lost control of Geists")
		}
	})
	t.Run("stolen then returned stays released", func(t *testing.T) {
		t.Parallel()
		e, geists, bear := geistsHold(t)
		e.emit(events.Event{Kind: events.ControlChange, Obj: geists, Player: 1})
		e.emit(events.Event{Kind: events.ControlChange, Obj: geists, Player: 0})
		if e.G.Obj(geists).Controller != 0 {
			t.Fatalf("precondition: Geists controller = %d, want 0", e.G.Obj(geists).Controller)
		}
		driveToStepAny(t, e, 2, 1, state.StepUpkeep)
		if e.G.Obj(bear).Tapped {
			t.Fatalf("the Bear was held again after control of Geists returned (CR 611.2b: the effect does not start again)")
		}
	})
}
