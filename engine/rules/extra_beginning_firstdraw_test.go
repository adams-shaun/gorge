package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestExtraBeginningPhaseTurnOneDrawSkipsOnlyTheDraw: CR 103.8a skips the
// starting player's first draw step only. Shadow of the Second Sun's extra
// beginning phase (ExtraPhase$ Beginning, spliced after Main2) on turn 1 of a
// two-player game still runs its untap and upkeep; the skipped draw then ends
// the extra phase and the walk resumes at the splice point's successor (the
// end step), never at Main1.
func TestExtraBeginningPhaseTurnOneDrawSkipsOnlyTheDraw(t *testing.T) {
	t.Parallel()
	e := newSeats(t, 2)
	driveToStep(t, e, 1, 0, state.StepMain2)
	if len(e.G.Players) != 2 || e.G.Turn != 1 {
		t.Fatalf("fixture: want turn 1 of a two-player game, got turn %d seats %d", e.G.Turn, len(e.G.Players))
	}
	upkeeps := stepEntries(e, 1, state.StepUpkeep)
	main1s := stepEntries(e, 1, state.StepMain1)
	e.emit(events.Event{Kind: events.ExtraPhase, Player: 0, Amount: 1,
		Step: state.StepMain2, IDs: []state.ObjID{state.ObjID(state.StepUntap)}})
	if len(e.G.ExtraPhases) != 1 {
		t.Fatalf("fixture: ExtraPhases = %+v, want the one grant", e.G.ExtraPhases)
	}
	driveToStep(t, e, 1, 0, state.StepEnd)
	if got := stepEntries(e, 1, state.StepUpkeep) - upkeeps; got != 1 {
		t.Fatalf("extra upkeep steps = %d, want 1 (only the draw is skipped)", got)
	}
	if got := stepEntries(e, 1, state.StepDraw); got != 0 {
		t.Fatalf("turn 1 entered the draw step %d time(s), want 0", got)
	}
	if got := stepEntries(e, 1, state.StepMain1) - main1s; got != 0 {
		t.Fatalf("the extra beginning phase jumped to Main1 %d time(s), want 0", got)
	}
	if len(e.G.ExtraPhases) != 0 {
		t.Fatalf("ExtraPhases not drained: %+v", e.G.ExtraPhases)
	}
}
