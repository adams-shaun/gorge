package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// phaseOutNow phases src out (CR 702.26b), as Vodalian Illusionist or
// Reality Ripple resolving in response would.
func phaseOutNow(t *testing.T, e *Engine, src state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.PhaseOut, Obj: src, Amount: 1})
	if o := e.G.Obj(src); o == nil || !o.PhasedOut || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the source did not phase out: %+v", o)
	}
}

// driveToEchoTrigger passes priority until the echo trigger of id is on top
// of the stack with a priority decision pending.
func driveToEchoTrigger(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	for i := 0; i < 4000 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if n := len(e.G.Stack); n > 0 && d.Kind == decision.KPriority {
			if top := e.G.Obj(e.G.Stack[n-1]); top != nil && top.Source == id {
				return
			}
		}
		switch {
		case d.Kind == decision.KChoose && d.Source == id:
			t.Fatalf("precondition: the echo election came before its trigger could be answered: %q", d.Prompt)
		case d.Kind == decision.KPriority:
			submitPass(t, e)
		case d.Kind == decision.KAttackers || d.Kind == decision.KBlockers:
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player}); err != nil {
				t.Fatalf("submit %s: %v", d.Kind, err)
			}
		case answerIfDiscard(t, e):
		default:
			t.Fatalf("unexpected decision %+v while waiting for the echo trigger", d)
		}
	}
	t.Fatal("precondition: the echo trigger never reached the stack")
}

// CR 702.30a, 118.12a, 702.26b: an echo trigger whose source phased out in
// response resolves as for a source that left: nothing is asked and nothing
// is sacrificed (dirac's ExtCumulative reads the phased-out source as gone).
func TestEchoPhasedOutSourceAsksNothing(t *testing.T) {
	t.Parallel()
	e, id := echoEntryEngine(t, "Cradle Guard", 3)
	driveToEchoTrigger(t, e, id)
	turn, active := e.G.Turn, e.G.Active
	if active != 0 || e.G.Step != state.StepUpkeep {
		t.Fatalf("precondition: the echo trigger is on the stack at turn %d seat %d step %s, want seat 0's upkeep", turn, active, e.G.Step)
	}
	phaseOutNow(t, e, id)
	if d := driveEchoQuiet(t, e, id, turn, active, state.StepDraw); d != nil {
		t.Fatalf("the echo of a phased-out source asked %q, want nothing", d.Prompt)
	}
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || !o.PhasedOut {
		t.Fatalf("Cradle Guard: zone %s phased out %v, want the battlefield, phased out", o.Zone, o.PhasedOut)
	}
}

// CR 702.32a, 702.26b: a fading trigger whose source phased out in response
// removes no fade counter (the source can't be affected) and sacrifices
// nothing.
func TestFadingPhasedOutSourceKeepsCounters(t *testing.T) {
	t.Parallel()
	e, id := blastoderm(t)
	e.G.Active, e.G.Priority = 0, 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	e.putTriggersOnStack()
	if n := len(e.G.Stack); n != 1 {
		t.Fatalf("precondition: the upkeep stacked %d triggers, want the fading trigger", n)
	}
	phaseOutNow(t, e, id)
	e.resolveTop()
	if len(e.G.Stack) != 0 {
		t.Fatalf("the fading trigger is still on the stack (%d objects)", len(e.G.Stack))
	}
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || !o.PhasedOut || o.Counter("FADE") != 3 {
		t.Fatalf("Blastoderm: zone %s phased out %v FADE %d, want the battlefield, phased out, 3", o.Zone, o.PhasedOut, o.Counter("FADE"))
	}
}
