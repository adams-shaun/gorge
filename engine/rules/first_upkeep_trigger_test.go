package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestParadoxHazeGrantsExactlyOneExtraUpkeep: Paradox Haze's trigger carries
// FirstUpkeep$ True ("enchanted player's FIRST upkeep each turn") and no
// Phase$. Unread, it fired at every step and again in the extra upkeep it had
// granted, so the grants never ended (hunt seed 90493: a 230-grant queue and
// a livelock). Haze enchants seat 0 from turn 1's main phase on: seat 1's
// turn 2 is untouched, seat 0's turn 3 runs exactly two upkeeps off exactly
// one grant, and the log replays.
func TestParadoxHazeGrantsExactlyOneExtraUpkeep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := addPhaseEngine(t, reg, []*cards.Card{lookup(t, reg, "Paradox Haze")}, nil)
	haze := moveByName(t, e, 0, "Paradox Haze", state.ZHand)
	stageRawEntry(t, e, haze)
	e.emit(events.Event{Kind: events.Attach, Obj: haze, Player: 0, Text: "attach to player"})
	if o := e.G.Obj(haze); o.Zone != state.ZBattlefield || !o.HasAttachedPlayer || o.AttachedPlayer != 0 {
		t.Fatalf("precondition: Haze zone=%v attachedPlayer=%v/%d, want on the battlefield enchanting seat 0",
			o.Zone, o.HasAttachedPlayer, o.AttachedPlayer)
	}
	e.pending = nil
	e.priorityRound()
	driveToStep(t, e, 3, 0, state.StepMain1)
	if got := e.G.UpkeepsThisTurn; got != 2 {
		t.Fatalf("UpkeepsThisTurn = %d in turn 3, want 2 (the ordinary upkeep + one extra)", got)
	}
	grants := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.ExtraPhase && ev.Amount > 0 })
	if grants != 1 {
		t.Fatalf("%d extra-phase grants by turn 3's main phase, want 1 (turn 1 is past its upkeep, turn 2 is seat 1's, the extra upkeep must not re-grant)", grants)
	}
	replayCheck(t, e, cfg)
}
