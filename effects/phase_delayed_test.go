package effects

// The delayed-trigger half of the shared Forge phase-name parser gate: the
// corpus's range and list Phase$ shapes on `DB$ DelayedTrigger` lines now
// register the FIRST set member the game will still reach
// (state.EarliestAfter -- Forge removes a delayed trigger the moment it
// fires, so even a multi-step value fires exactly once, at the first listed
// phase still ahead), and an unresolvable value still records the
// unrecognized-phase Note instead of firing at the wrong phase. The old
// delayedPhaseStep substring switch checked `main2` before `main1`, so it
// mapped `Main1,Main2` to Main2 and had no notion of ranges.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestDelayedTriggerRegistersEarliestSetMemberStillAhead(t *testing.T) {
	for _, tc := range []struct {
		phase string
		cur   state.Step
		want  state.Step
	}{
		// The open range `Upkeep->` (5 raw corpus DelayedTrigger lines): the
		// first of Upkeep..Cleanup still ahead of the current step.
		{"Upkeep->", state.StepUntap, state.StepUpkeep},
		{"Upkeep->", state.StepUpkeep, state.StepDraw},
		{"Upkeep->", state.StepCombatDamage, state.StepEndCombat},
		// The list form (`Main1,Main2`, 6 raw lines): "the next main phase"
		// registered from combat or Main1 is Main2, not next turn's Main1.
		{"Main1,Main2", state.StepCombatDamage, state.StepMain2},
		{"Main1,Main2", state.StepMain1, state.StepMain2},
		{"Main1,Main2", state.StepUntap, state.StepMain1},
		// A closed range registers its first member still ahead.
		{"BeginCombat->EndCombat", state.StepDeclareAttackers, state.StepDeclareBlockers},
		// Single-step values keep the exact behaviour the old parser had.
		{"End of Turn", state.StepMain2, state.StepEnd},
		{"End of Turn", state.StepEnd, state.StepEnd}, // next turn's end step
		{"Upkeep", state.StepMain2, state.StepUpkeep},
	} {
		h := newHost(t, 2)
		h.g.Step = tc.cur
		Resolve(h, &Ctx{Controller: 0, Source: 1},
			sa(t, "DB$ DelayedTrigger | Mode$ Phase | Phase$ "+tc.phase+" | Execute$ X"))
		if len(h.log) != 1 || h.log[0].Kind != events.DelayedRegister {
			t.Fatalf("Phase$ %s at %s: log = %+v", tc.phase, tc.cur, h.log)
		}
		if got := h.log[0].Step; got != tc.want {
			t.Errorf("Phase$ %s registered at %s: step = %s, want %s", tc.phase, tc.cur, got, tc.want)
		}
	}
}

// TestDelayedTriggerUnknownPhaseStillNotes pins the reporting half: a value
// no Forge script name resolves (the bare "End" the old substring parser
// accepted) records the unrecognized-phase Note and registers nothing.
func TestDelayedTriggerUnknownPhaseStillNotes(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0, Source: 1},
		sa(t, "DB$ DelayedTrigger | Mode$ Phase | Phase$ End | Execute$ X"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note {
		t.Fatalf("log = %+v, want one Note", h.log)
	}
	if got := h.log[0].Text; !strings.Contains(got, "unrecognized phase End") {
		t.Fatalf("Note text = %q, want it to name the unrecognized phase", got)
	}
}
