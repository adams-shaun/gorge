package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 514.3a: the cleanup step's state-based check follows its 514.2
// actions. A Grizzly Bears with two -1/-1 counters kept alive by Giant
// Growth's +3/+3 is a 0/0 once the pump ends at cleanup, so it dies IN that
// cleanup step (and the active player then gets priority), not at the next
// turn's first state-based check.
func TestCleanupChecksStateBasedActionsAfterEndingEffects(t *testing.T) {
	e, _ := fdnCensusEngine(t, "Grizzly Bears", "Giant Growth")
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	gg := moveByName(t, e, 0, "Giant Growth", state.ZHand)
	e.pending = nil
	e.probe(func() {
		effects.Resolve(e, &effects.Ctx{Source: gg, Controller: 0,
			Targets: []state.Target{{Obj: bears}}}, e.G.Obj(gg).Face().Abilities[0])
	})
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		for _, o := range d.Options {
			if o.Obj == bears {
				submitChoices(t, e, o.Index)
				break
			}
		}
	}
	if got := e.Toughness(bears); got != 5 {
		t.Fatalf("precondition: Giant Growth left the Bears at toughness %d, want 5", got)
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: bears, Counter: "M1M1", Amount: 2})
	e.pending = nil
	e.Advance()
	if z := e.G.Obj(bears).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: pumped Bears with two -1/-1 counters in %s", z)
	}
	turn := e.G.Turn
	var diedTurn int32 = -1
	var diedStep state.Step
	sawCleanupPriority := false
	for i := 0; i < 200 && diedTurn < 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision")
		}
		if d.Kind == decision.KPriority && e.G.Step == state.StepCleanup {
			sawCleanupPriority = true
		}
		if d.Kind == decision.KPriority {
			submitPass(t, e)
		} else {
			fdnCensusAnswerMin(t, e, d)
		}
		if e.G.Obj(bears).Zone != state.ZBattlefield {
			diedTurn, diedStep = e.G.Turn, e.G.Step
		}
	}
	if diedTurn != turn || diedStep != state.StepCleanup {
		t.Fatalf("the Bears died on turn %d step %s, want turn %d's cleanup step", diedTurn, diedStep, turn)
	}
	for i := 0; i < 5 && !sawCleanupPriority; i++ {
		if d := e.Pending(); d != nil && d.Kind == decision.KPriority && e.G.Step == state.StepCleanup {
			sawCleanupPriority = true
		}
	}
	if !sawCleanupPriority {
		t.Fatal("no cleanup-step priority after the state-based action (CR 514.3a)")
	}
}

// CR 514.3a, 704.5j: Oko, the Ringleader copies a creature until end of
// turn; a second Oko arriving after combat is no duplicate until the copy
// ends at cleanup, where the legend rule asks which to keep. The performed
// action gives the active player priority in that cleanup step, as any other
// cleanup state-based action does, even though the keep choice is answered
// before the step repeats.
func TestCleanupLegendRuleAnswerGivesActivePlayerPriority(t *testing.T) {
	e, _ := fdnCensusEngine(t, "Oko, the Ringleader", "Oko, the Ringleader", "Grizzly Bears")
	first := moveByName(t, e, 0, "Oko, the Ringleader", state.ZBattlefield)
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	e.pending = nil
	e.Advance()
	second := state.ObjID(0)
	kept, sawPriority := false, false
	for i := 0; i < 400 && !e.G.Over && !sawPriority; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision")
		}
		switch {
		case d.Kind == decision.KPriority && kept:
			if e.G.Step != state.StepCleanup {
				t.Fatalf("after the legend rule's choice the next priority is in %s, want the cleanup step (CR 514.3a)", e.G.Step)
			}
			sawPriority = true
		case d.Kind == decision.KPriority:
			if e.G.Step == state.StepMain2 && second == 0 {
				second = moveByName(t, e, 0, "Oko, the Ringleader", state.ZBattlefield)
				e.pending = nil
				e.Advance()
				continue
			}
			submitPass(t, e)
		case d.Kind == decision.KTarget:
			picked := false
			for _, o := range d.Options {
				if o.Obj == bears {
					submitChoices(t, e, o.Index)
					picked = true
					break
				}
			}
			if !picked {
				fdnCensusAnswerMin(t, e, d)
			}
		case d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "keep":
			if e.G.Step != state.StepCleanup {
				t.Fatalf("legend rule asked in %s, want the cleanup step", e.G.Step)
			}
			submitChoices(t, e, 0)
			kept = true
		default:
			fdnCensusAnswerMin(t, e, d)
		}
	}
	if !kept || !sawPriority {
		t.Fatalf("kept=%v sawPriority=%v (first Oko %d, second %d): the scenario did not reach the cleanup legend rule", kept, sawPriority, first, second)
	}
}
