package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Phase$ spellings the old substring matcher never admitted: the corpus
// spells one step several ways, and only spellings that happened to be
// substrings of the hyphenated step names ("main1", "upkeep") ever fired.
// "End of Turn" alone is 927 trigger lines across 921 corpus files; every
// one of those triggers was permanently silent. phaseNamesStep matches by
// canonical token instead.
func TestPhaseNamesStepAliasTable(t *testing.T) {
	cases := []struct {
		phase string
		step  state.Step
		want  bool
	}{
		{"End of Turn", state.StepEnd, true},
		{"End Of Turn", state.StepEnd, true}, // the capitalised variant, 24 lines
		{"End", state.StepEnd, true},
		{"End of Turn", state.StepEndCombat, false},
		{"BeginCombat", state.StepBeginCombat, true},
		{"EndCombat", state.StepEndCombat, true},
		{"Main", state.StepMain1, true},
		{"Main", state.StepMain2, true},
		{"Main1,Main2", state.StepMain1, true},
		{"Main1,Main2", state.StepMain2, true},
		{"Main1,Main2", state.StepUpkeep, false},
		{"Main1", state.StepMain2, false},
		{"Upkeep", state.StepUpkeep, true},
		{"Upkeep->", state.StepUpkeep, true}, // arrow idiom: fires, repeat unmodelled
		{"Upkeep->", state.StepUntap, false},
		{"BeginCombat->EndCombat", state.StepBeginCombat, false}, // range idiom: fail closed
		{"Declare Attackers", state.StepDeclareAttackers, true},
		{"Draw", state.StepDraw, true},
		{"Untap", state.StepUntap, true},
		{"Cleanup", state.StepCleanup, true},
		{"Combat", state.StepCombatDamage, true},
		{"Mausoleum", state.StepEnd, false}, // unknown spelling fails closed
	}
	for _, tc := range cases {
		if got := phaseNamesStep(tc.phase, tc.step); got != tc.want {
			t.Errorf("phaseNamesStep(%q, %v) = %t, want %t", tc.phase, tc.step, got, tc.want)
		}
	}
}

// TestEndOfTurnTriggerFires drives the live shape the alias table fixed:
// Kuroki's real "At the beginning of your end step" trigger — Phase$
// "End of Turn", the 927-line spelling — reaches its placement target ask
// when seat 0's end step begins, instead of never firing.
func TestEndOfTurnTriggerFires(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := stealEngine(t, 741)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Kuroki, Thief of Talents"))
	driveToStep(t, e, e.G.Turn, 0, state.StepEnd)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Player != 0 {
		t.Fatalf("pending = %+v, want Kuroki's end-step placement target ask", d)
	}
}
