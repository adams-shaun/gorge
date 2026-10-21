package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// murkDwellersPump resolves Murk Dwellers' real "+2/+0 until end of combat"
// trigger body directly (the AttackerUnblocked dispatcher is not under test)
// and returns the creature's id.
func murkDwellersPump(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	a := crAbortMove(t, e, 0, "Murk Dwellers", state.ZBattlefield)
	f := e.G.Obj(a).Face()
	if f.PT != "2/2" || len(f.Triggers) != 1 || f.Triggers[0].Effect == nil ||
		f.Triggers[0].Effect.Params["Duration"] != "UntilEndOfCombat" || f.Triggers[0].Effect.Params["NumAtt"] != "+2" {
		t.Fatal("CR 511.2 Murk Dwellers seq 0: real until-end-of-combat +2/+0 fixture changed")
	}
	e.resolveAbility(a, 0, nil, f.Triggers[0].Effect, f.SVars)
	return a
}

// TestCR511UntilEndCombatAppliesFromCreation: CR 511.2 says an "until end of
// combat" effect EXPIRES at the end of the combat phase; it does not wait for
// combat to begin. One created in the precombat main phase applies at once
// and lasts through the combat that follows.
func TestCR511UntilEndCombatAppliesFromCreation(t *testing.T) {
	t.Parallel()
	e := crResolutionEngine(t, []string{"Murk Dwellers"}, nil)
	// The step walk below stands in for advanceStep, which runs only once
	// every player has passed: no decision is pending at a step boundary.
	e.pending = nil
	if e.G.Step != state.StepMain1 {
		t.Fatalf("CR 511.2 seq %d: fixture starts at %s, want precombat main", len(e.L.Events), e.G.Step)
	}
	a := murkDwellersPump(t, e)
	if got := e.Power(a); got != 4 {
		t.Errorf("CR 511.2 Murk Dwellers seq %d: power %d in precombat main right after the pump resolved; want 4", len(e.L.Events), got)
	}
	e.setStep(state.StepBeginCombat)
	if got := e.Power(a); got != 4 {
		t.Errorf("CR 511.2 Murk Dwellers seq %d: power %d at beginning of combat; want 4", len(e.L.Events), got)
	}
}

// TestCR511UntilEndCombatDoesNotReturnInExtraCombat: the effect ends when the
// combat phase it was alive in ends, so a second combat phase the same turn
// (Relentless Assault, an AddPhase grant) does not bring it back.
func TestCR511UntilEndCombatDoesNotReturnInExtraCombat(t *testing.T) {
	t.Parallel()
	e := crResolutionEngine(t, []string{"Murk Dwellers"}, nil)
	// The step walk below stands in for advanceStep, which runs only once
	// every player has passed: no decision is pending at a step boundary.
	e.pending = nil
	e.setStep(state.StepBeginCombat)
	a := murkDwellersPump(t, e)
	e.setStep(state.StepEndCombat)
	if got := e.Power(a); got != 4 {
		t.Fatalf("CR 511.2 Murk Dwellers seq %d: power %d during end of combat; want 4", len(e.L.Events), got)
	}
	e.setStep(state.StepMain2)
	if got := e.Power(a); got != 2 {
		t.Fatalf("CR 511.2 Murk Dwellers seq %d: power %d in postcombat main; want 2", len(e.L.Events), got)
	}
	// An additional combat phase this turn.
	e.setStep(state.StepBeginCombat)
	if got := e.Power(a); got != 2 {
		t.Errorf("CR 511.2 Murk Dwellers seq %d: power %d in an extra combat phase after the first combat ended; want printed 2", len(e.L.Events), got)
	}
	e.setStep(state.StepDeclareAttackers)
	if got := e.Power(a); got != 2 {
		t.Errorf("CR 511.2 Murk Dwellers seq %d: power %d at the extra combat's declare attackers; want printed 2", len(e.L.Events), got)
	}
}
