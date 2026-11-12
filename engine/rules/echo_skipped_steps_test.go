package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEchoSkippedDrawStepRecordsUpkeep: with Necropotence skipping seat 0's
// draw step, an echo paid at turn 3's upkeep must not be asked again at
// turn 5's (CR 702.30a). The record is the upkeep's end, not the Draw
// step's entry, which a skipped draw never reaches.
func TestEchoSkippedDrawStepRecordsUpkeep(t *testing.T) {
	t.Parallel()
	e, guide, necro := echoEntryEngineWith(t, "Karmic Guide", "Necropotence", 5)
	if o := e.G.Obj(necro); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Necropotence is not on the battlefield: %+v", o)
	}
	driveEchoQuiet(t, e, guide, 3, 0, state.StepDraw)
	d := tapUntilElection(t, e, guide, 8)
	if d.Options[0].Kind != "echo_pay" {
		t.Fatalf("election before paying = %+v", d.Options)
	}
	submitChoices(t, e, 0)
	lib0 := len(e.G.Zone(state.ZLibrary, 0))
	if d := driveEchoQuiet(t, e, guide, 3, 0, state.StepMain1); d != nil {
		t.Fatalf("unexpected election leaving turn 3's upkeep: %+v", d.Options)
	}
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != lib0 {
		t.Fatalf("precondition: seat 0's library went %d -> %d, the draw step was not skipped", lib0, got)
	}
	if d := driveEchoQuiet(t, e, guide, 5, 0, state.StepMain1); d != nil {
		t.Fatalf("echo asked again at turn 5 after paying at turn 3 (skipped draw): LastUpkeepTurn=%d options=%+v",
			e.G.Players[0].LastUpkeepTurn, d.Options)
	}
	if e.G.Obj(guide).Zone != state.ZBattlefield {
		t.Fatalf("guide left the battlefield: %s", e.G.Obj(guide).Zone)
	}
}

// TestEchoSkippedUpkeepStepStillOwed: with Eon Hub skipping upkeeps, a
// permanent acquired after the previous (real) upkeep is still owed echo at
// the next upkeep that actually happens (CR 702.30a).
func TestEchoSkippedUpkeepStepStillOwed(t *testing.T) {
	t.Parallel()
	e, guide, hub := echoEntryEngineWith(t, "Karmic Guide", "Eon Hub", 5)
	if o := e.G.Obj(hub); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Eon Hub is not on the battlefield: %+v", o)
	}
	if d := driveEchoQuiet(t, e, guide, 2, 1, state.StepMain1); d != nil {
		t.Fatalf("unexpected election before turn 3: %+v", d.Options)
	}
	mark := len(e.L.Events)
	if d := driveEchoQuiet(t, e, guide, 3, 0, state.StepMain1); d != nil {
		t.Fatalf("election during a skipped upkeep: %+v", d.Options)
	}
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.StepChange && ev.Step == state.StepUpkeep {
			t.Fatalf("precondition: an Upkeep StepChange was logged in turn 3 under Eon Hub")
		}
	}
	// Remove the Hub so turn 5's upkeep happens.
	e.emit(events.Event{Kind: events.MoveZone, Obj: hub, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
	d := driveEchoQuiet(t, e, guide, 5, 0, state.StepMain1)
	if d == nil {
		t.Fatalf("no echo election at turn 5 after the skipped upkeep: LastUpkeepTurn=%d", e.G.Players[0].LastUpkeepTurn)
	}
	if len(d.Options) < 2 || d.Options[0].Kind != "echo_pay" || d.Options[1].Kind != "echo_sac" {
		t.Fatalf("election = %+v", d.Options)
	}
}
