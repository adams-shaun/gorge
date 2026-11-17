package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// aegisGrant casts Aegis Angel for seat 0 against seat 1's Ud Bear, targets
// the Bear with the enters trigger, and returns the engine with both ids once
// the trigger has resolved.
func aegisGrant(t *testing.T) (e *Engine, angel, bear state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	aa, ok := reg.Lookup("Aegis Angel")
	if !ok {
		t.Fatalf("corpus fixture: Aegis Angel missing")
	}
	cfg := seatZeroStart(Config{Seed: 4243, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{aa}, mountainDeck(t, 39)...),
			append([]*cards.Card{card(t, udBear)}, mountainDeck(t, 39)...),
		},
		Tokens: map[string]*cards.Card{},
	})
	e = New(cfg)
	e.Advance()
	bear = moveByName(t, e, 1, "Ud Bear", state.ZBattlefield)
	angel = moveByName(t, e, 0, "Aegis Angel", state.ZHand)
	addMana(t, e, 0, "WWWWWW")
	submitChoices(t, e, castOptionFor(t, e, angel).Index)
	d := passPriorityUntilNonPriority(t, e)
	if d.Kind != decision.KTarget {
		t.Fatalf("precondition: want the enters trigger's target ask, got %v", d.Kind)
	}
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
	passUntilStackEmpty(t, e, 40)
	if e.G.Obj(angel).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Aegis Angel is not on the battlefield")
	}
	if !e.HasKeyword(bear, "Indestructible") {
		t.Fatalf("precondition: the Bear did not gain indestructible from the enters trigger")
	}
	return e, angel, bear
}

// TestPumpAsLongAsControlOutlivesCleanup pins CR 611.2b for a Pump with
// Duration$ AsLongAsControl (Aegis Angel): the grant survives end-of-turn
// cleanup and ends only when its source leaves, changes controller or phases
// out.
func TestPumpAsLongAsControlOutlivesCleanup(t *testing.T) {
	t.Parallel()
	t.Run("survives cleanup and the next turn", func(t *testing.T) {
		t.Parallel()
		e, _, bear := aegisGrant(t)
		driveToStepAny(t, e, 2, 1, state.StepUpkeep)
		if !e.HasKeyword(bear, "Indestructible") {
			t.Fatalf("the grant ended at cleanup while Aegis Angel is still controlled")
		}
		driveToStepAny(t, e, 3, 0, state.StepUpkeep)
		if !e.HasKeyword(bear, "Indestructible") {
			t.Fatalf("the grant ended on a later turn while Aegis Angel is still controlled")
		}
	})
	t.Run("ends when Aegis Angel leaves", func(t *testing.T) {
		t.Parallel()
		e, angel, bear := aegisGrant(t)
		e.emit(events.Event{Kind: events.MoveZone, Obj: angel, From: state.ZBattlefield, To: state.ZGraveyard, Player: 0})
		if e.G.Obj(angel).Zone != state.ZGraveyard {
			t.Fatalf("precondition: Angel zone = %v", e.G.Obj(angel).Zone)
		}
		if e.HasKeyword(bear, "Indestructible") {
			t.Fatalf("the grant outlived Aegis Angel leaving the battlefield")
		}
	})
	t.Run("ends when the opponent gains control", func(t *testing.T) {
		t.Parallel()
		e, angel, bear := aegisGrant(t)
		e.emit(events.Event{Kind: events.ControlChange, Obj: angel, Player: 1})
		if e.G.Obj(angel).Controller != 1 {
			t.Fatalf("precondition: Angel controller = %d, want 1", e.G.Obj(angel).Controller)
		}
		if e.HasKeyword(bear, "Indestructible") {
			t.Fatalf("the grant outlived Aegis Angel changing controller")
		}
		e.emit(events.Event{Kind: events.ControlChange, Obj: angel, Player: 0})
		if e.HasKeyword(bear, "Indestructible") {
			t.Fatalf("the grant restarted when control of Aegis Angel returned")
		}
	})
	t.Run("ends when Aegis Angel phases out", func(t *testing.T) {
		t.Parallel()
		e, angel, bear := aegisGrant(t)
		e.emit(events.Event{Kind: events.PhaseOut, Obj: angel, Amount: 1})
		if !e.G.Obj(angel).PhasedOut {
			t.Fatalf("precondition: Angel did not phase out")
		}
		if e.HasKeyword(bear, "Indestructible") {
			t.Fatalf("the grant outlived Aegis Angel phasing out")
		}
		e.emit(events.Event{Kind: events.PhaseOut, Obj: angel, Amount: -1})
		if e.HasKeyword(bear, "Indestructible") {
			t.Fatalf("the grant restarted when Aegis Angel phased back in")
		}
	})
}
