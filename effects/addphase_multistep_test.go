package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The DB$ AddPhase multi-step-value tests (ticket ap1's "support a
// multi-step set" half; rules/addphase_multistep_test.go owns the turn-walk
// side). effAddPhase resolves ExtraPhase$/AfterPhase$/FollowedBy$ through
// the ONE shared phase-name parser (state.ParsePhases), so a value naming a
// whole phase -- a comma list or an A->B range -- resolves instead of
// loud-degrading:
//
//   - ExtraPhase$ multi-step: the extra phase runs the whole named phase
//     (entry = its first step, range end = its last, the RANGEEND rider the
//     fold reads -- the entry's own default range is otherwise wrong);
//   - AfterPhase$ multi-step: the splice point is the phase's END (its last
//     step -- the grant waits until the whole named phase has run);
//   - FollowedBy$ multi-step: the resume point is the phase's BEGINNING
//     (its first step -- the walk enters the followed phase).

// addPhaseEventsOf returns the ExtraPhase grant events the fake host logged.
func addPhaseEventsOf(h *fakeHost) []events.Event {
	var out []events.Event
	for _, e := range h.log {
		if e.Kind == events.ExtraPhase && e.Amount > 0 {
			out = append(out, e)
		}
	}
	return out
}

// TestAddPhaseMultiStepExtraPhaseRidesRangeEndRider: ExtraPhase$ "Upkeep,Draw"
// resolves to the whole two-step phase -- entry Upkeep, range end Draw, the
// range carried in the RANGEEND rider because the entry's own default
// (state.ExtraPhaseRangeEnd) is Upkeep only. Precondition: the two values
// actually differ, so the rider is load-bearing.
func TestAddPhaseMultiStepExtraPhaseRidesRangeEndRider(t *testing.T) {
	if state.ExtraPhaseRangeEnd(state.StepUpkeep) == state.StepDraw {
		t.Fatal("precondition: ExtraPhaseRangeEnd(Upkeep) must differ from Draw for the rider to matter")
	}
	g, _ := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "DB$ AddPhase | ExtraPhase$ Upkeep,Draw"))
	if notes := notesOf(h); len(notes) != 0 {
		t.Fatalf("multi-step ExtraPhase$ loud-degraded: %v", notes)
	}
	evs := addPhaseEventsOf(h)
	if len(evs) != 1 {
		t.Fatalf("ExtraPhase grant events = %d, want 1 (log %+v)", len(evs), h.log)
	}
	ev := evs[0]
	if got := state.Step(ev.IDs[0]); got != state.StepUpkeep {
		t.Fatalf("entry step = %s, want Upkeep", got)
	}
	riders := events.DecodeExtraPhaseRiders(ev.Text)
	if !riders.HasRangeEnd || riders.RangeEnd != state.StepDraw {
		t.Fatalf("RANGEEND rider = %+v, want Draw", riders)
	}
}

// TestAddPhaseMultiStepAfterPhaseAndFollowedByResolve: AfterPhase$
// "Untap->Draw" (the whole beginning phase as a range) splices after the
// phase's END (Draw, not the current step) and FollowedBy$ "Main" resumes
// at Main1, its first step. Precondition: Draw is not the fake game's
// current step, so the asserted splice point comes from the parsed value,
// not the omitted form.
func TestAddPhaseMultiStepAfterPhaseAndFollowedByResolve(t *testing.T) {
	g, _ := board(t)
	if g.Step == state.StepDraw {
		t.Fatal("precondition: the fixture game must not already stand at Draw")
	}
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "DB$ AddPhase | ExtraPhase$ Combat | AfterPhase$ Untap->Draw | FollowedBy$ Main"))
	if notes := notesOf(h); len(notes) != 0 {
		t.Fatalf("multi-step AfterPhase$/FollowedBy$ loud-degraded: %v", notes)
	}
	evs := addPhaseEventsOf(h)
	if len(evs) != 1 {
		t.Fatalf("ExtraPhase grant events = %d, want 1 (log %+v)", len(evs), h.log)
	}
	ev := evs[0]
	if state.Step(ev.Step) != state.StepDraw {
		t.Fatalf("splice point = %s, want Draw (Beginning's last step)", state.Step(ev.Step))
	}
	if len(ev.IDs) != 2 || state.Step(ev.IDs[0]) != state.StepBeginCombat ||
		state.Step(ev.IDs[1]) != state.StepMain1 {
		t.Fatalf("IDs = %v, want [BeginCombat, Main1] (entry, Main's first step)", ev.IDs)
	}
}

// TestAddPhaseNonContiguousExtraPhaseFailsClosed: a multi-step set with a
// gap ("Untap,Main2") has no contiguous Entry..RangeEnd walk to store in the
// queue, so it fails closed -- one loud Note, no grant event.
func TestAddPhaseNonContiguousExtraPhaseFailsClosed(t *testing.T) {
	if state.StepMain2 <= state.StepUntap+1 {
		t.Fatal("precondition: Main2 must not be adjacent to Untap for the set to have a gap")
	}
	g, _ := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "DB$ AddPhase | ExtraPhase$ Untap,Main2"))
	if evs := addPhaseEventsOf(h); len(evs) != 0 {
		t.Fatalf("a non-contiguous set granted %d extra phase(s): %+v", len(evs), evs)
	}
	if notes := notesOf(h); len(notes) != 1 {
		t.Fatalf("Notes = %v, want exactly one loud Note", notes)
	}
}
