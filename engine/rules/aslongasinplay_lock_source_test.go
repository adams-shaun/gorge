package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// tidebinderHold seats Tidebinder Mage under seat 0 against seat 1's tapped Ud
// Bear and returns the engine, the Mage and the Bear once the enters trigger
// (an Effect with ReplacementEffects$ DontUntap, Duration$ AsLongAsInPlay) has
// resolved.
func tidebinderHold(t *testing.T) (e *Engine, mage, bear state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	tm, ok := reg.Lookup("Tidebinder Mage")
	if !ok {
		t.Fatalf("corpus fixture: Tidebinder Mage missing")
	}
	cfg := seatZeroStart(Config{Seed: 4243, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{tm}, mountainDeck(t, 39)...),
			append([]*cards.Card{card(t, udBear), card(t, udBear)}, mountainDeck(t, 38)...),
		},
		Tokens: map[string]*cards.Card{},
	})
	e = New(cfg)
	e.Advance()
	bear = moveByName(t, e, 1, "Ud Bear", state.ZBattlefield)
	mage = moveByName(t, e, 0, "Tidebinder Mage", state.ZHand)
	addMana(t, e, 0, "UU")
	submitChoices(t, e, castOptionFor(t, e, mage).Index)
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
		t.Fatalf("precondition: the Mage's enters trigger did not tap the Bear")
	}
	return e, mage, bear
}

// TestAsLongAsInPlayLockEndsWithSource pins CR 611.2b / 400.7: the untap lock
// of Duration$ AsLongAsInPlay ends when its source leaves the battlefield, and
// the same ObjID returning is a new object that does not revive it.
func TestAsLongAsInPlayLockEndsWithSource(t *testing.T) {
	t.Parallel()
	t.Run("source in play holds the creature", func(t *testing.T) {
		t.Parallel()
		e, mage, bear := tidebinderHold(t)
		driveToStepAny(t, e, 2, 1, state.StepUpkeep)
		if e.G.Obj(mage).Zone != state.ZBattlefield {
			t.Fatalf("precondition: the Mage is in zone %v", e.G.Obj(mage).Zone)
		}
		if !e.G.Obj(bear).Tapped {
			t.Fatalf("the Bear untapped while the Mage is still in play")
		}
	})
	t.Run("source bounced and returned does not revive the lock", func(t *testing.T) {
		t.Parallel()
		e, mage, bear := tidebinderHold(t)
		inc := e.G.Obj(mage).Incarnation
		e.emit(events.Event{Kind: events.MoveZone, Obj: mage, From: state.ZBattlefield, To: state.ZHand})
		// The returned Mage's own enters trigger locks a second Bear, so the
		// original Bear is held only if the OLD lock came back.
		bear2 := moveByName(t, e, 1, "Ud Bear", state.ZBattlefield)
		e.emit(events.Event{Kind: events.MoveZone, Obj: mage, From: state.ZHand, To: state.ZBattlefield})
		if d := passPriorityUntilNonPriority(t, e); d.Kind == decision.KTarget {
			pick := -1
			for _, o := range d.Options {
				if o.Obj == bear2 {
					pick = o.Index
				}
			}
			if pick < 0 {
				t.Fatalf("the second Bear was not offered as a target: %+v", d.Options)
			}
			submitChoices(t, e, pick)
		}
		passUntilStackEmpty(t, e, 40)
		if !e.G.Obj(bear2).Tapped {
			t.Fatalf("precondition: the returned Mage did not tap the second Bear")
		}
		if o := e.G.Obj(mage); o.Zone != state.ZBattlefield || o.Incarnation == inc {
			t.Fatalf("precondition: the Mage must be back as a new incarnation (zone %v, inc %d -> %d)", o.Zone, inc, o.Incarnation)
		}
		driveToStepAny(t, e, 2, 1, state.StepUpkeep)
		if e.G.Obj(bear).Tapped {
			t.Fatalf("the Bear stayed tapped: the old lock came back with the recast Mage (CR 611.2b)")
		}
		if !e.G.Obj(bear2).Tapped {
			t.Fatalf("the second Bear untapped: the new Mage's own lock must hold")
		}
	})
}
