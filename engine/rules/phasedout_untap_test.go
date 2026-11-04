package rules

// CR 702.26b, 502.3 -- a phased-out permanent is treated as though it does
// not exist, so its controller's untap action leaves it tapped. Normally a
// permanent phases in just before that action and untaps with the rest; one a
// CantPhaseIn keeps out (Disciple of Caelus Nin's printed "Permanents can't
// phase in.", The City on the Edge of Forever's token) must stay tapped.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestPhasedOutPermanentStaysTappedThroughUntap(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 1596, "Disciple of Caelus Nin", "Grizzly Bears")
	bears := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	e.emit(events.Event{Kind: events.Tap, Obj: bears})
	e.emit(events.Event{Kind: events.PhaseOut, Obj: bears, Amount: 1})
	if o := e.G.Obj(bears); o == nil || !o.PhasedOut || !o.Tapped {
		t.Fatalf("precondition: the Bears are not tapped and phased out: %+v", o)
	}
	moveSeededCard(t, e, 0, cr702corpusCard(t, "Disciple of Caelus Nin"), state.ZBattlefield)
	addMana(t, e, 0, "")
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority && len(e.G.Stack) == 0 {
			break
		}
		if d.Kind == decision.KPriority {
			passPriorityOnce(t, e)
			continue
		}
		var keep []int
		for _, o := range d.Options {
			if d.Max == 0 || len(keep) < d.Max {
				keep = append(keep, o.Index)
			}
		}
		submitChoices(t, e, keep...)
	}
	if len(e.activeStatics(effects.ModeCantPhaseIn)) == 0 {
		t.Fatal("precondition: Disciple's printed CantPhaseIn static is not active")
	}
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	o := e.G.Obj(bears)
	if o == nil || !o.PhasedOut {
		t.Fatalf("precondition: the Bears phased in under \"Permanents can't phase in.\": %+v", o)
	}
	if !o.Tapped {
		t.Fatal("CR 502.3, 702.26b: the untap step untapped a permanent that is still phased out")
	}
	replayCheck(t, e, cfg)
}
