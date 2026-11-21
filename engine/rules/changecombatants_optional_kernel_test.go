package rules

// The per-attacker "you may reselect" confirm of ChangeCombatants Optional$ True.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const optionalReselectSA = "DB$ ChangeCombatants | Defined$ Valid Creature.attacking | Optional$ True | Attacking$ True"

func optionalReselectBoard(t *testing.T) (*Engine, state.ObjID, *decision.Decision) {
	t.Helper()
	e := fixtureEngine(t, 3)
	a1 := placeSrc(t, e, 0, "Name:Vanquisher\nTypes:Creature\nPT:2/2\nOracle:x\n", state.ZBattlefield)
	e.G.Obj(a1).IsAttacking = true
	e.G.Obj(a1).Attacking = 1
	d := runProbeSA(t, e, parseFixtureSA(t, optionalReselectSA), func() *effects.Ctx {
		return &effects.Ctx{Source: a1, Controller: 0}
	}, nil)
	if d == nil || d.ResumeKind != "changecombatants_optional" || len(d.Options) != 2 || d.Player != 0 {
		t.Fatalf("first ask = %+v, want the seat-0 changecombatants_optional yes/no", d)
	}
	if d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
		t.Fatalf("options = %+v, want yes then no", d.Options)
	}
	return e, a1, d
}

// TestChangeCombatantsOptionalDeclineKeepsDefenderKernel: answering no poses
// nothing more, emits no CombatRetarget or Note, and the attack stays put.
func TestChangeCombatantsOptionalDeclineKeepsDefenderKernel(t *testing.T) {
	t.Parallel()
	e, a1, _ := optionalReselectBoard(t)
	start := len(e.L.Events)
	if next := answerAsk(t, e, 1); next != nil {
		t.Fatalf("declining posed another ask: %+v", next)
	}
	evs := eventsSince(e, start)
	if got := retargetEvents(evs); len(got) != 0 {
		t.Fatalf("decline emitted %+v", got)
	}
	if n := countEventKind(evs, events.Note); n != 0 {
		t.Fatalf("decline emitted %d note(s): %+v", n, evs)
	}
	if o := e.G.Obj(a1); !o.IsAttacking || o.Attacking != 1 {
		t.Fatalf("attacker = attacking %v at seat %d, want still seat 1", o.IsAttacking, o.Attacking)
	}
}

// TestChangeCombatantsOptionalYesAsksDefenderKernel: answering yes poses the
// defender ask; picking seat 2 emits exactly one CombatRetarget.
func TestChangeCombatantsOptionalYesAsksDefenderKernel(t *testing.T) {
	t.Parallel()
	e, a1, _ := optionalReselectBoard(t)
	start := len(e.L.Events)
	d := answerAsk(t, e, 0)
	if d == nil || d.ResumeKind != "choice" || len(d.Options) != 2 {
		t.Fatalf("after yes, ask = %+v, want the defender choice", d)
	}
	if got := retargetEvents(eventsSince(e, start)); len(got) != 0 {
		t.Fatalf("emitted %+v before the defender answer", got)
	}
	if next := answerAsk(t, e, optionIdxForPlayer(t, d, 2)); next != nil {
		t.Fatalf("a further ask was posed: %+v", next)
	}
	got := retargetEvents(eventsSince(e, start))
	if len(got) != 1 || got[0].Obj != a1 || got[0].Player != 2 {
		t.Fatalf("retarget events = %+v, want one naming %d -> seat 2", got, a1)
	}
}
