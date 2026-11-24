package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestOgreSavantManaSpentInterveningIf: CR 603.4. Ogre Savant's enters trigger
// reads "if {U} was spent to cast it" on the Execute body, which is the
// intervening-if the trigger must check before it goes on the stack. Cast with
// red mana only it never triggers (no target ask); with one blue it triggers
// and returns the target creature.
func TestOgreSavantManaSpentInterveningIf(t *testing.T) {
	t.Parallel()

	// Red only: the condition is false at the event, so no trigger and no
	// target decision. The creature still enters.
	e := handEngine(t, corpusAlternativeCard(t, "Ogre Savant"))
	bear := e.G.AddObject(corpusAlternativeCard(t, "Silvercoat Lion"), 1)
	bear.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{bear.ID})
	e.G.Players[0].Pool[state.MR] = 5
	spell := e.G.Zone(state.ZHand, 0)[0]
	castMode(t, e, spell, "")
	if got := len(e.G.Stack); got != 1 {
		t.Fatalf("precondition: stack has %d objects after the cast, want the creature spell", got)
	}
	e.resolveTop()
	if z := e.G.Obj(spell).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: Ogre Savant is in %s after resolution, want battlefield", z)
	}
	e.priorityRound()
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		t.Fatalf("red-only Ogre Savant asked for a target: %+v", d)
	}
	if got := len(e.pendingTriggers); got != 0 {
		t.Fatalf("red-only Ogre Savant queued %d trigger(s), want 0", got)
	}
	if got := len(e.G.Stack); got != 0 {
		t.Fatalf("red-only Ogre Savant put %d trigger(s) on the stack, want 0", got)
	}
	if z := e.G.Obj(bear.ID).Zone; z != state.ZBattlefield {
		t.Fatalf("red-only Ogre Savant moved the bear to %s, want it to stay", z)
	}

	// One blue: the condition holds at the event, so the trigger goes on the
	// stack and its target is returned to hand.
	e = handEngine(t, corpusAlternativeCard(t, "Ogre Savant"))
	bear = e.G.AddObject(corpusAlternativeCard(t, "Silvercoat Lion"), 1)
	bear.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{bear.ID})
	e.G.Players[0].Pool[state.MR], e.G.Players[0].Pool[state.MU] = 4, 1
	spell = e.G.Zone(state.ZHand, 0)[0]
	castMode(t, e, spell, "")
	e.resolveTop()
	if got := len(e.pendingTriggers); got != 1 {
		t.Fatalf("blue-spent Ogre Savant queued %d trigger(s), want 1", got)
	}
	e.priorityRound()
	submitTarget(t, e, bear.ID)
	e.resolveTop()
	if z := e.G.Obj(bear.ID).Zone; z != state.ZHand {
		t.Fatalf("blue-spent Ogre Savant left the bear in %s, want hand", z)
	}
}

// TestSteamcoreWeirdManaSpentInterveningIf: the same body-carried
// ConditionManaSpent$ on Steamcore Weird's enters trigger ("if {R} was spent
// to cast it") is an intervening-if. Paid with blue only it neither triggers
// nor deals damage or asks for a target; with red spent it deals 2.
func TestSteamcoreWeirdManaSpentInterveningIf(t *testing.T) {
	t.Parallel()

	// UUUU: no red was spent, so the trigger never fires.
	e := handEngine(t, corpusAlternativeCard(t, "Steamcore Weird"))
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("precondition: seat 1 life = %d, want 20", life)
	}
	e.G.Players[0].Pool[state.MU] = 4
	spell := e.G.Zone(state.ZHand, 0)[0]
	castMode(t, e, spell, "")
	if got := len(e.G.Stack); got != 1 {
		t.Fatalf("precondition: stack has %d objects after the cast, want the creature spell", got)
	}
	e.resolveTop()
	if got := len(e.pendingTriggers); got != 0 {
		t.Fatalf("blue-only Steamcore Weird queued %d trigger(s), want 0", got)
	}
	e.priorityRound()
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		t.Fatalf("blue-only Steamcore Weird asked for a target: %+v", d)
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("blue-only Steamcore Weird dealt damage: seat 1 life = %d, want 20", life)
	}

	// One red: the condition holds, so the trigger targets and deals 2.
	e = handEngine(t, corpusAlternativeCard(t, "Steamcore Weird"))
	e.G.Players[0].Pool[state.MU], e.G.Players[0].Pool[state.MR] = 3, 1
	spell = e.G.Zone(state.ZHand, 0)[0]
	castMode(t, e, spell, "")
	e.resolveTop()
	if got := len(e.pendingTriggers); got != 1 {
		t.Fatalf("red-spent Steamcore Weird queued %d trigger(s), want 1", got)
	}
	e.priorityRound()
	submitTargetOnSeat(t, e, 1)
	e.resolveTop()
	if life := e.G.Players[1].Life; life != 18 {
		t.Fatalf("red-spent Steamcore Weird left seat 1 at %d life, want 18", life)
	}
}
