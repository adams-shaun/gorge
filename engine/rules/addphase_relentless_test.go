package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// stepsAfter lists the StepChange steps in e's log from event index from.
func stepsAfter(e *Engine, from int) []state.Step {
	var out []state.Step
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.StepChange {
			out = append(out, ev.Step)
		}
	}
	return out
}

// TestRelentlessAssaultFirstMainTurnShape: "After this main phase, there is an
// additional combat phase followed by an additional main phase" cast in the
// FIRST main phase puts both phases before the ordinary combat -- precombat
// main, extra combat, extra main, ordinary combat, postcombat main. The grant
// is the one effAddPhase makes for the card (splice Main1, entry BeginCombat,
// FollowedBy$ Main2); cast in the second main phase the extra main is the
// last one before the end step (TestAggravatedAssaultExtraCombatFollowedByMain2).
func TestRelentlessAssaultFirstMainTurnShape(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := addPhaseEngine(t, reg, []*cards.Card{lookup(t, reg, "Relentless Assault")}, nil)
	driveToTurn(t, e, 3, 0)
	if e.G.Step != state.StepMain1 {
		t.Fatalf("step %s, want Main1 (the grant's splice point)", e.G.Step)
	}
	e.emit(events.Event{Kind: events.ExtraPhase, Player: 0, Amount: 1, Step: state.StepMain1,
		IDs: []state.ObjID{state.ObjID(state.StepBeginCombat), state.ObjID(state.StepMain2)}})
	from := len(e.L.Events)
	driveToStep(t, e, 3, 0, state.StepEnd)
	got := stepsAfter(e, from)
	want := []state.Step{
		state.StepBeginCombat, state.StepDeclareAttackers, state.StepEndCombat, // the extra combat (no attackers)
		state.StepMain2,                                                        // the additional main phase
		state.StepBeginCombat, state.StepDeclareAttackers, state.StepEndCombat, // the ordinary combat
		state.StepMain2, state.StepEnd,
	}
	// Declare attackers with no creature is skipped by the walk; compare
	// without it so the check does not depend on that jump.
	got = slices.DeleteFunc(got, func(s state.Step) bool { return s == state.StepDeclareAttackers })
	want = slices.DeleteFunc(want, func(s state.Step) bool { return s == state.StepDeclareAttackers })
	if !slices.Equal(got, want) {
		t.Fatalf("steps after the grant = %v, want %v", got, want)
	}
	if len(e.G.ExtraPhases) != 0 {
		t.Fatalf("ExtraPhases not drained: %+v", e.G.ExtraPhases)
	}
	replayCheck(t, e, cfg)
}
