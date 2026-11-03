package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 117.3b, 514.3a: after a cleanup-step trigger resolves, the active player
// receives priority, and the cleanup step repeats only once all players pass
// in succession with the stack empty. The cleanup discard of an over-limit
// hand fires the opponent's Tinybones, Bauble Burglar; once that trigger
// resolves the active player must be asked for priority in the SAME cleanup
// step (lockstep pair 138: gorge used to begin the next turn without asking).
func TestCleanupTriggerResolutionGivesActivePlayerPriority(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	tiny := mustCorpusCard(t, reg, "Tinybones, Bauble Burglar")
	e := corpusEngine(t, reg, []*cards.Card{tiny}, []*cards.Card{tiny})
	ap := e.G.Active
	opp := e.G.NextAlive(ap)
	moveByName(t, e, opp, "Tinybones, Bauble Burglar", state.ZBattlefield)
	for len(e.G.Zone(state.ZHand, ap)) < 8 {
		lib := e.G.Zone(state.ZLibrary, ap)
		e.emit(events.Event{Kind: events.MoveZone, Obj: lib[len(lib)-1], From: state.ZLibrary, To: state.ZHand})
	}
	turn := e.G.Turn

	// Pass to this turn's cleanup discard.
	for i := 0; ; i++ {
		if i > 100 {
			t.Fatal("never reached the cleanup discard")
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision")
		}
		if d.Kind == decision.KChoose && e.G.Step == state.StepCleanup {
			break
		}
		if d.Kind == decision.KPriority {
			submitPass(t, e)
		} else {
			fdnCensusAnswerMin(t, e, d)
		}
	}
	fdnCensusAnswerMin(t, e, e.Pending())
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %d after the cleanup discard, want Tinybones' trigger", len(e.G.Stack))
	}
	// Both players pass; the trigger resolves.
	for i := 0; len(e.G.Stack) > 0; i++ {
		if i > 10 {
			t.Fatal("the trigger never resolved")
		}
		submitPass(t, e)
	}
	d := e.Pending()
	if e.G.Turn != turn || e.G.Step != state.StepCleanup {
		t.Fatalf("after the cleanup trigger resolved: turn %d step %s, want turn %d's cleanup step with priority", e.G.Turn, e.G.Step, turn)
	}
	if d == nil || d.Kind != decision.KPriority || d.Player != ap {
		t.Fatalf("after the cleanup trigger resolved: pending %+v, want the active player's (seat %d) priority (CR 117.3b)", d, ap)
	}
	// Everyone passes on the empty stack: the cleanup step repeats (CR
	// 514.3b) and, with nothing more to do, the next turn begins.
	submitPass(t, e)
	if d := e.Pending(); d != nil && d.Kind == decision.KPriority && e.G.Turn == turn {
		if d.Player != opp || e.G.Step != state.StepCleanup {
			t.Fatalf("after the active player's pass: seat %d step %s, want seat %d in cleanup", d.Player, e.G.Step, opp)
		}
		submitPass(t, e)
	}
	if e.G.Turn != turn+1 {
		t.Fatalf("after both passes on the empty stack: turn %d, want %d", e.G.Turn, turn+1)
	}
}

// CR 514.3a: a trigger waiting at the cleanup step is put on the stack and the
// active player then gets priority, even when the trigger is removed at once
// because no mode is left to choose (CR 603.3c). Monument to Endurance has
// already offered all three of its modes this turn when the cleanup discard
// fires it a fourth time.
func TestCleanupTriggerRemovedForNoModeStillGivesPriority(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	mon := mustCorpusCard(t, reg, "Monument to Endurance")
	e := corpusEngine(t, reg, []*cards.Card{mon}, []*cards.Card{mon})
	ap := e.G.Active
	id := moveByName(t, e, ap, "Monument to Endurance", state.ZBattlefield)
	for _, m := range []string{"DBDraw", "DBToken", "DBLoseLife"} {
		e.G.Obj(id).ModeChoices = append(e.G.Obj(id).ModeChoices,
			state.ModeChoice{Mode: m, Scope: state.ModeScopeThisTurn, Turn: e.G.Turn})
	}
	for len(e.G.Zone(state.ZHand, ap)) < 8 {
		lib := e.G.Zone(state.ZLibrary, ap)
		e.emit(events.Event{Kind: events.MoveZone, Obj: lib[len(lib)-1], From: state.ZLibrary, To: state.ZHand})
	}
	turn := e.G.Turn
	for i := 0; ; i++ {
		if i > 100 {
			t.Fatal("never reached the cleanup discard")
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision")
		}
		if d.Kind == decision.KChoose && e.G.Step == state.StepCleanup {
			break
		}
		if d.Kind == decision.KPriority {
			submitPass(t, e)
		} else {
			fdnCensusAnswerMin(t, e, d)
		}
	}
	fdnCensusAnswerMin(t, e, e.Pending())
	d := e.Pending()
	if e.G.Turn != turn || e.G.Step != state.StepCleanup || len(e.G.Stack) != 0 {
		t.Fatalf("after the discard: turn %d step %s stack %d, want turn %d's cleanup with an empty stack", e.G.Turn, e.G.Step, len(e.G.Stack), turn)
	}
	if d == nil || d.Kind != decision.KPriority || d.Player != ap {
		t.Fatalf("after the discard: pending %+v, want the active player's priority (CR 514.3a)", d)
	}
}
