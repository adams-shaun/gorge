package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Amber Prison's lock is a DB$ Effect carrying a bodyless Layer$ CantHappen
// Untap replacement with Duration$ UntilUntaps: "for as long as Amber Prison
// remains tapped" (CR 611.2b). Inline copy of the printed script.
const amberPrisonSrc = "Name:Amber Prison\nManaCost:4\nTypes:Artifact\n" +
	"A:AB$ Tap | Cost$ 4 T | ValidTgts$ Creature,Artifact | SubAbility$ DBLock | SpellDescription$ Tap target.\n" +
	"SVar:DBLock:DB$ Effect | Duration$ UntilUntaps | RememberObjects$ Targeted | ReplacementEffects$ NoUntap\n" +
	"SVar:NoUntap:Event$ Untap | Layer$ CantHappen | ValidCard$ Card.IsRemembered | ValidStepTurnToController$ You | Description$ x\n" +
	"Oracle:x\n"

// amberPrisonBoard puts Amber Prison (seat 0) and an opposing Bear (seat 1) on
// the battlefield and resolves the prison's ability on the Bear with the prison
// tapped as the {T} cost leaves it.
func amberPrisonBoard(t *testing.T) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	prison := onBoard(t, e, 0, amberPrisonSrc)
	bear := onBoard(t, e, 1, "Name:Lock Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Tap, Obj: prison})
	face := e.G.Obj(prison).Face()
	effects.Resolve(e, &effects.Ctx{Source: prison, Controller: 0, SVars: face.SVars,
		Targets: []state.Target{{Obj: bear}}}, face.Abilities[0])
	if !e.G.Obj(bear).Tapped || !e.G.Obj(prison).Tapped {
		t.Fatalf("precondition: bear tapped=%v prison tapped=%v", e.G.Obj(bear).Tapped, e.G.Obj(prison).Tapped)
	}
	// layerEngine starts with no decision pending; open the priority round the
	// driver passes through.
	e.pending = nil
	e.priorityRound()
	return e, prison, bear
}

func TestAmberPrisonLockEndsWhenHostUntaps(t *testing.T) {
	t.Parallel()
	e, prison, bear := amberPrisonBoard(t)
	e.emit(events.Event{Kind: events.Untap, Obj: prison})
	if e.G.Obj(prison).Tapped {
		t.Fatal("precondition: prison still tapped")
	}
	driveToStep(t, e, 2, 1, state.StepMain1)
	if e.G.Obj(bear).Tapped {
		t.Fatal("the creature stayed tapped through its untap step after Amber Prison untapped")
	}
}

func TestAmberPrisonLockHoldsWhileHostTapped(t *testing.T) {
	t.Parallel()
	e, prison, bear := amberPrisonBoard(t)
	driveToStep(t, e, 2, 1, state.StepMain1)
	if !e.G.Obj(prison).Tapped {
		t.Fatal("precondition: prison untapped during the opponent's turn")
	}
	if !e.G.Obj(bear).Tapped {
		t.Fatal("the creature untapped while Amber Prison stayed tapped")
	}
}
