package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestSkipPhaseStepGrantSkipsOneStep: a named-step SkipPhase grant (Step Untap,
// scope SkipScopeNext) arms the seat's one-shot untap bit; the consuming event
// (Amount -1) clears it, so only the first untap step is skipped. IDs empty is
// still the all-combat grant, unchanged.
func TestSkipPhaseStepGrantSkipsOneStep(t *testing.T) {
	g := state.NewGame([]string{"Ann", "Bob"})
	Apply(g, Event{Kind: SkipPhase, Player: 1, Step: state.StepUntap, IDs: []state.ObjID{state.SkipScopeNext}})
	if got := g.PhaseSkips[1]; got != state.SkipUntapNext {
		t.Fatalf("PhaseSkips[1] = %d after the grant, want SkipUntapNext (%d)", got, state.SkipUntapNext)
	}
	if len(g.PhaseSkips) != 1 {
		t.Fatalf("PhaseSkips = %v, want only seat 1", g.PhaseSkips)
	}
	Apply(g, Event{Kind: SkipPhase, Player: 1, Step: state.StepUntap, Amount: -1})
	if len(g.PhaseSkips) != 0 {
		t.Fatalf("PhaseSkips = %v after the untap step was skipped, want it spent", g.PhaseSkips)
	}
	Apply(g, Event{Kind: SkipPhase, Player: 0, Step: state.StepDraw, IDs: []state.ObjID{state.SkipScopeNext}})
	Apply(g, Event{Kind: SkipPhase, Player: 0, Step: state.StepBeginCombat})
	if got, want := g.PhaseSkips[0], state.SkipDrawNext|state.SkipCombatPending; got != want {
		t.Fatalf("PhaseSkips[0] = %d, want draw-next | all-combat pending (%d)", got, want)
	}
	Apply(g, Event{Kind: SkipPhase, Player: 0, Step: state.StepDraw, Amount: -1})
	if got := g.PhaseSkips[0]; got != state.SkipCombatPending {
		t.Fatalf("PhaseSkips[0] = %d after the draw skip, want the all-combat grant kept", got)
	}
}

// TestSkipPhaseOnceSkipsTheNextCombatPhaseOnly: the one-combat grant is its own
// bit (not the all-combat one), survives to the seat's turn when scoped Next,
// is consumed by one Amount -1 event, and a this-turn grant expires at the
// TurnChange.
func TestSkipPhaseOnceSkipsTheNextCombatPhaseOnly(t *testing.T) {
	g := state.NewGame([]string{"Ann", "Bob"})
	Apply(g, Event{Kind: SkipPhase, Player: 1, Step: state.StepBeginCombat, IDs: []state.ObjID{state.SkipScopeNext}})
	Apply(g, Event{Kind: SkipPhase, Player: 0, Step: state.StepBeginCombat, IDs: []state.ObjID{state.SkipScopeTurn}})
	if g.PhaseSkips[1] != state.SkipCombatOnce || g.PhaseSkips[0] != state.SkipCombatTurn {
		t.Fatalf("PhaseSkips = %v, want seat 1 once, seat 0 this-turn", g.PhaseSkips)
	}
	if g.CombatSkipped(1) || g.CombatSkipped(0) {
		t.Fatal("a one-shot grant is not the all-combat live skip")
	}
	Apply(g, Event{Kind: TurnChange, Player: 0, Amount: 2})
	if g.PhaseSkips[0] != 0 {
		t.Fatalf("PhaseSkips[0] = %d after the turn changed, want the this-turn grant dropped", g.PhaseSkips[0])
	}
	if g.PhaseSkips[1] != state.SkipCombatOnce {
		t.Fatalf("PhaseSkips[1] = %d after the turn changed, want the once grant kept", g.PhaseSkips[1])
	}
	Apply(g, Event{Kind: SkipPhase, Player: 1, Step: state.StepBeginCombat, Amount: -1})
	if len(g.PhaseSkips) != 0 {
		t.Fatalf("PhaseSkips = %v after the combat was skipped, want it spent", g.PhaseSkips)
	}
}
