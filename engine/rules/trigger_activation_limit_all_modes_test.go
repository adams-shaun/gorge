package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggerActivationLimitHonouredOnEveryMode: "This ability triggers only
// once each turn" (ActivationLimit$ on a T: line) used to be honoured only
// for the actionTriggerModes set, so a line of any other mode -- LifeLost,
// ChangesZone, SpellCast, CounterAdded(Once), ... (72 corpus lines) --
// triggered every time. LifeLost is not in that set; its second same-turn
// match must queue nothing, and the next turn re-arms it.
func TestTriggerActivationLimitHonouredOnEveryMode(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	if actionTriggerModes["LifeLost"] {
		t.Fatal("precondition: LifeLost must be outside actionTriggerModes for this test to mean anything")
	}
	id := onBoard(t, e, 0, "Name:Loser\nTypes:Creature\n"+
		"T:Mode$ LifeLost | ValidPlayer$ Opponent | TriggerZones$ Battlefield | ActivationLimit$ 1 | Execute$ TrigDraw\n"+
		"SVar:TrigDraw:DB$ Draw\nOracle:x\n")
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	if n := queuedPhaseTriggers(e, id); n != 1 {
		t.Fatalf("first loss queued %d triggers, want 1", n)
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	if n := queuedPhaseTriggers(e, id); n != 1 {
		t.Fatalf("second same-turn loss: %d queued, want still 1 (ActivationLimit$ 1)", n)
	}
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.TurnChange, Player: 1, Amount: e.G.Turn + 1})
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	if n := queuedPhaseTriggers(e, id); n != 1 {
		t.Fatalf("loss on the next turn queued %d, want 1 (the limit is per turn)", n)
	}
}

// TestTriggerLimitLooksBackAtTheDepartingSource: a "one or more creatures you
// control die ... only once each turn" line that has triggered this turn does
// not trigger again on its own source's death, which is the same object looking
// back (CR 603.10a), not the new one CR 400.7 makes it afterwards. A source that
// has not triggered does, and the next turn re-arms the line.
func TestTriggerLimitLooksBackAtTheDepartingSource(t *testing.T) {
	t.Parallel()
	const face = "Name:Watcher\nTypes:Creature\nPT:2/2\n" +
		"T:Mode$ ChangesZoneAll | TriggerZones$ Battlefield | ValidCards$ Creature.YouCtrl | Origin$ Battlefield | Destination$ Graveyard | ActivationLimit$ 1 | Execute$ TrigDraw\n" +
		"SVar:TrigDraw:DB$ Draw\nOracle:x\n"
	const bear = "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"
	die := func(e *Engine, id state.ObjID) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	}
	t.Run("spent", func(t *testing.T) {
		e := layerEngine(t)
		w, b := onBoard(t, e, 0, face), onBoard(t, e, 0, bear)
		die(e, b)
		if n := queuedPhaseTriggers(e, w); n != 1 {
			t.Fatalf("the other creature's death queued %d, want 1", n)
		}
		die(e, w)
		if n := queuedPhaseTriggers(e, w); n != 1 {
			t.Fatalf("its own death queued %d in all, want still 1", n)
		}
	})
	t.Run("unspent", func(t *testing.T) {
		e := layerEngine(t)
		w := onBoard(t, e, 0, face)
		die(e, w)
		if n := queuedPhaseTriggers(e, w); n != 1 {
			t.Fatalf("its own first death queued %d, want 1", n)
		}
	})
	t.Run("next turn", func(t *testing.T) {
		e := layerEngine(t)
		w, b := onBoard(t, e, 0, face), onBoard(t, e, 0, bear)
		die(e, b)
		e.pendingTriggers = nil
		e.emit(events.Event{Kind: events.TurnChange, Player: 1, Amount: e.G.Turn + 1})
		die(e, w)
		if n := queuedPhaseTriggers(e, w); n != 1 {
			t.Fatalf("its own death a turn later queued %d, want 1", n)
		}
	})
}

// TestExemplarOfLightDrawTriggersOncePerTurnAndOnlyForYou pins both halves
// of Exemplar of Light's "Whenever you put one or more +1/+1 counters on
// this creature, draw a card. This ability triggers only once each turn."
// (Mode$ CounterAddedOnce | ValidSource$ You | ActivationLimit$ 1): the
// limit (CounterAddedOnce was outside actionTriggerModes, so it was
// ignored) and the putter (ValidSource$ was not read by the CounterAdded
// matcher, so an opponent's Battlegrowth drew a card; CR 109.5).
func TestExemplarOfLightDrawTriggersOncePerTurnAndOnlyForYou(t *testing.T) {
	t.Parallel()
	put := func(e *Engine, id state.ObjID, adder state.PlayerID) {
		prev := e.SetCounterAdder(adder)
		e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1})
		e.SetCounterAdder(prev)
	}
	e := layerEngine(t)
	ex := onBoardCard(t, e, 0, corpusCard(t, "Exemplar of Light"))
	put(e, ex, 0)
	if n := queuedPhaseTriggers(e, ex); n != 1 {
		t.Fatalf("you put a counter: %d triggers queued, want the draw", n)
	}
	put(e, ex, 0)
	if n := queuedPhaseTriggers(e, ex); n != 1 {
		t.Fatalf("second counter the same turn: %d queued, want still 1 (triggers only once each turn)", n)
	}
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.TurnChange, Player: 1, Amount: e.G.Turn + 1})
	put(e, ex, 0)
	if n := queuedPhaseTriggers(e, ex); n != 1 {
		t.Fatalf("counter on a later turn: %d queued, want 1", n)
	}

	e2 := layerEngine(t)
	ex2 := onBoardCard(t, e2, 0, corpusCard(t, "Exemplar of Light"))
	put(e2, ex2, 1)
	if n := queuedPhaseTriggers(e2, ex2); n != 0 {
		t.Fatalf("an opponent put the counter: %d queued, want none (ValidSource$ You)", n)
	}
	// An unattributed placement (no adder in flight) fails closed, and it
	// does not consume the turn's one use.
	e2.emit(events.Event{Kind: events.CounterChange, Obj: ex2, Counter: "P1P1", Amount: 1})
	if n := queuedPhaseTriggers(e2, ex2); n != 0 {
		t.Fatalf("unattributed placement: %d queued, want none", n)
	}
	put(e2, ex2, 0)
	if n := queuedPhaseTriggers(e2, ex2); n != 1 {
		t.Fatalf("you put a counter after the rejected ones: %d queued, want 1", n)
	}
}
