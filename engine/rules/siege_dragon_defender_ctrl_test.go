package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// siegeDragonAttack attacks seat 1 with the real Siege Dragon (seat 0) and
// reports how many TriggerPush events its declaration produced and whether a
// trigger-order decision was raised. wall places a Wall under seat 1.
func siegeDragonAttack(t *testing.T, wall bool) (pushes int, ordered bool, e *Engine, dragon state.ObjID) {
	t.Helper()
	e = combatEngine(t)
	dragon = onBoardCard(t, e, 0, mshCorpusCard(t, "Siege Dragon"))
	e.G.Obj(dragon).SummonSick = false
	if wall {
		w := onBoard(t, e, 1, "Name:Test Wall\nManaCost:1\nTypes:Creature Wall\nPT:0/4\nOracle:x\n")
		if !e.matchesSpec("Wall", w, e.specCtx(dragon, 0)) {
			t.Fatalf("precondition: the placed permanent %d is not a Wall", w)
		}
	}
	start := len(e.L.Events)
	e.askAttackers()
	submitAttackerAt(t, e, dragon, 1)
	if d := e.Pending(); d != nil && d.Kind == decision.KTriggerOrder {
		ordered = true
	}
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.TriggerPush && ev.Obj == dragon {
			pushes++
		}
	}
	return pushes, ordered, e, dragon
}

// TestSiegeDragonNoTriggerThroughWalls: CR 603.4 -- "if defending player
// controls no Walls" is checked as the attack is declared, with the defending
// player bound. Unbound, DefenderCtrl matched nothing and the trigger queued
// (then fizzled at resolution).
func TestSiegeDragonNoTriggerThroughWalls(t *testing.T) {
	t.Parallel()
	pushes, ordered, e, _ := siegeDragonAttack(t, true)
	if pushes != 0 || ordered {
		t.Fatalf("with a Wall: trigger pushes = %d, trigger-order asked = %v; want none", pushes, ordered)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("with a Wall: stack has %d objects, want 0", len(e.G.Stack))
	}

	pushes, _, e, _ = siegeDragonAttack(t, false)
	if pushes != 1 {
		t.Fatalf("no Wall: trigger pushes = %d, want 1", pushes)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("no Wall: stack has %d objects, want 1", len(e.G.Stack))
	}
}
