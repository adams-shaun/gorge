package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func delayedModeWatcher(t testing.TB) (*Engine, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	src := onBoard(t, e, 0, "Name:Delayed watcher\nTypes:Creature Wizard\nPT:2/2\n"+
		"SVar:Trig:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n")
	return e, src
}

func registerInlineDelayed(t testing.TB, e *Engine, src state.ObjID, mode, clauses string) {
	t.Helper()
	e.emit(events.Event{Kind: events.DelayedRegister, Obj: src, Player: 0,
		Step: e.G.Step, Counter: "Trig", Text: mode + ":Mode$ " + mode + clauses})
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].EventMode != mode {
		t.Fatalf("registration = %+v, want one %s registration", e.G.Delayed, mode)
	}
}

func TestDelayedTriggerDeadRegistrationIsCollected(t *testing.T) {
	e, src := delayedModeWatcher(t)
	e.emit(events.Event{Kind: events.DelayedRegister, Obj: src, Player: 0,
		Step: e.G.Step, Counter: "MissingSVar"})
	if len(e.G.Delayed) != 1 {
		t.Fatalf("precondition: delayed registration = %+v", e.G.Delayed)
	}
	e.emit(events.Event{Kind: events.StepChange, Step: e.G.Step})
	if len(e.G.Delayed) != 0 {
		t.Fatalf("dead registration was not collected: %+v", e.G.Delayed)
	}
	found := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedRemove {
			found = true
		}
	}
	if !found {
		t.Fatal("collection did not emit DelayedRemove")
	}
}

func TestDelayedTriggerDeadEventRegistrationCollectedBeforeMatch(t *testing.T) {
	e, src := delayedModeWatcher(t)
	registerInlineDelayed(t, e, src, "ChangesZone", " | ValidCard$ Creature")
	e.G.Obj(src).Face().SVars = map[string]string{}
	other := onBoard(t, e, 1, "Name:Artifact\nTypes:Artifact\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: other, From: state.ZBattlefield, To: state.ZGraveyard})
	if len(e.G.Delayed) != 0 {
		t.Fatalf("missing Execute registration survived unmatched event: %+v", e.G.Delayed)
	}
}

func TestDelayedTriggerEventModesFire(t *testing.T) {
	t.Run("ChangesZone", func(t *testing.T) {
		e, src := delayedModeWatcher(t)
		registerInlineDelayed(t, e, src, "ChangesZone", " | Origin$ Library | Destination$ Battlefield | ValidCard$ Creature")
		card := e.G.AddObject(card(t, "Name:Arrival\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
		if card.Zone != state.ZLibrary {
			t.Fatalf("precondition: arrival zone = %s, want library", card.Zone)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: card.ID, From: state.ZLibrary, To: state.ZBattlefield})
		if card.Zone != state.ZBattlefield || len(e.pendingTriggers) != 1 {
			t.Fatalf("zone delayed trigger did not fire: zone=%s pending=%d", card.Zone, len(e.pendingTriggers))
		}
	})

	t.Run("ChangesController", func(t *testing.T) {
		e, src := delayedModeWatcher(t)
		registerInlineDelayed(t, e, src, "ChangesController", " | ValidCard$ Creature | ValidOriginalController$ Player | ValidPlayer$ You")
		wrong := onBoard(t, e, 1, "Name:Wrong player\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		if e.G.Obj(wrong).Controller != 1 {
			t.Fatalf("precondition: wrong-player controller = %d, want 1", e.G.Obj(wrong).Controller)
		}
		e.emit(events.Event{Kind: events.ControlChange, Obj: wrong, Player: 0})
		if len(e.pendingTriggers) != 0 {
			t.Fatalf("ValidPlayer$ admitted the wrong original controller: pending=%d", len(e.pendingTriggers))
		}
		card := onBoard(t, e, 0, "Name:Borrowed\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		if e.G.Obj(card).Controller != 0 {
			t.Fatalf("precondition: controller = %d, want 0", e.G.Obj(card).Controller)
		}
		e.emit(events.Event{Kind: events.ControlChange, Obj: card, Player: 1})
		if e.G.Obj(card).Controller != 1 || len(e.pendingTriggers) != 1 {
			t.Fatalf("control-change delayed trigger did not fire: controller=%d pending=%d", e.G.Obj(card).Controller, len(e.pendingTriggers))
		}
	})

	t.Run("DamageDone", func(t *testing.T) {
		e, src := delayedModeWatcher(t)
		registerInlineDelayed(t, e, src, "DamageDone", " | ValidSource$ Creature | ValidTarget$ Player")
		e.dmgSrcOverride = src
		e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
		if e.G.Players[1].Life >= 20 || len(e.pendingTriggers) != 1 {
			t.Fatalf("damage delayed trigger did not fire: life=%d pending=%d", e.G.Players[1].Life, len(e.pendingTriggers))
		}
	})

	t.Run("AttackersDeclared", func(t *testing.T) {
		e, src := delayedModeWatcher(t)
		registerInlineDelayed(t, e, src, "AttackersDeclared", " | AttackingPlayer$ You | ValidAttackers$ Creature")
		attacker := onBoard(t, e, 0, "Name:Attacker\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		if !e.IsCreature(attacker) || e.G.Obj(attacker).Zone != state.ZBattlefield {
			t.Fatalf("precondition: attacker is not a battlefield creature")
		}
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{attacker}})
		if len(e.pendingTriggers) != 1 {
			t.Fatalf("attackers-declared delayed trigger did not fire: pending=%d", len(e.pendingTriggers))
		}
	})
}
