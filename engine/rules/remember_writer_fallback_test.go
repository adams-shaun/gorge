package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// thorAttacks declares The Mighty Thor as the only attacker and answers his
// trigger's "up to one target" ask with target, or none when target is zero,
// through to the trigger's resolution.
func thorAttacks(t *testing.T, target func(*Engine) state.ObjID) (*Engine, state.ObjID) {
	t.Helper()
	e := New(Config{Seed: 717, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	thor := onBoardCard(t, e, 0, mshCorpusCard(t, "The Mighty Thor, Jane Foster"))
	e.G.Obj(thor).SummonSick = false
	var tgt state.ObjID
	if target != nil {
		tgt = target(e)
	}
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	submitAttackersOnly(t, e, thor)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 0 {
		t.Fatalf("want Thor's up-to-one target ask, got %+v", d)
	}
	if tgt == 0 {
		submitChoices(t, e)
	} else {
		idx := -1
		for _, o := range d.Options {
			if o.Obj == tgt {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("target %d not offered: %+v", tgt, d.Options)
		}
		submitChoices(t, e, idx)
	}
	passUntilStackEmpty(t, e, 8)
	return e, thor
}

func movesOf(e *Engine, id state.ObjID) int {
	return countKind(e.L.Events, events.MoveZone, id)
}

// With no target chosen, "return that card" names nothing: the return must not
// read the trigger's capture, Thor himself, as the card to move.
func TestRememberWriterThatWroteNothingMovesNothing(t *testing.T) {
	t.Parallel()
	e, thor := thorAttacks(t, nil)
	if o := e.G.Obj(thor); movesOf(e, thor) != 0 || o.Zone != state.ZBattlefield || !o.IsAttacking {
		t.Fatalf("Thor made %d moves, zone %v, attacking %v; want none and still attacking", movesOf(e, thor), o.Zone, o.IsAttacking)
	}
}

// With a target, the chain's own memory is what returns, tapped; Thor stays.
func TestRememberWriterReturnsWhatItWrote(t *testing.T) {
	t.Parallel()
	var bear state.ObjID
	e, thor := thorAttacks(t, func(e *Engine) state.ObjID {
		bear = onBoard(t, e, 1, "Name:Fixture Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		return bear
	})
	if o := e.G.Obj(thor); movesOf(e, thor) != 0 || !o.IsAttacking {
		t.Fatalf("Thor made %d moves, attacking %v; want none and still attacking", movesOf(e, thor), o.IsAttacking)
	}
	if o := e.G.Obj(bear); o.Zone != state.ZBattlefield || !o.Tapped {
		t.Fatalf("bear = zone %v tapped %v, want it back tapped", o.Zone, o.Tapped)
	}
	if n := movesOf(e, bear); n != 2 {
		t.Fatalf("bear made %d moves, want exile and return", n)
	}
}

// A body naming Remembered with no earlier link that remembers still reads the
// capture: that is its only referent.
func TestRememberedCaptureStaysAFallbackWithoutAWriter(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	onBoard(t, e, 0, "Name:Pump Watcher\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/1\n"+
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other | Execute$ Pump\n"+
		"SVar:Pump:DB$ Pump | Defined$ Remembered | NumAtt$ +1 | NumDef$ +1\nOracle:x\n")
	bear := e.G.AddObject(card(t, "Name:Fixture Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	bear.Zone = state.ZHand
	id := bear.ID
	e.G.SetZone(state.ZHand, 0, []state.ObjID{id})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	e.resolveTop()
	if got := e.Power(id); got != 3 {
		t.Fatalf("pumped power = %d, want 3", got)
	}
}
