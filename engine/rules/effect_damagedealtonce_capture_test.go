package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const captureBearSrc = "Name:Capture Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
const captureWallSrc = "Name:Capture Wall\nManaCost:1 W\nTypes:Creature Wall\nPT:0/4\nOracle:x\n"

// playCombatThrough answers the target ask with the bear, the optional
// trigger ask with "yes" (option 0) and every priority ask with pass, until
// combat is over (the clock is past the combat-damage step, stack empty).
func playCombatThrough(t *testing.T, e *Engine, bear state.ObjID) {
	t.Helper()
	for i := 0; i < 80; i++ {
		if e.G.Over || (e.G.Step >= state.StepEndCombat && len(e.G.Stack) == 0) {
			return
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while driving combat (step %v)", e.G.Step)
		}
		switch d.Kind {
		case decision.KTarget:
			chooseTargetOption(t, e, bear)
		case decision.KBlockers:
			submitChoices(t, e)
		default:
			submitChoices(t, e, 0)
		}
	}
	t.Fatal("combat did not finish within the decision budget")
}

func TestLongRiverLurkerFlickersTheRememberedCreatureOnCombatDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg, []string{"Long River Lurker"}, []string{captureBearSrc},
		nil, []string{captureWallSrc})
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	bear := findBattlefield(t, e, 0, "Capture Bear", 0)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: bear not on the battlefield: %+v", o)
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	e.askAttackers()
	submitAttackers(t, e, bear)
	playCombatThrough(t, e, bear)

	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("defender life = %d, want 18 (the unblocked 2/2 dealt damage)", got)
	}
	// The engine keeps the ObjID across a zone change, so "a new object" is
	// read off the log: one exile MoveZone then one battlefield MoveZone, in
	// that order, after the attack, and the returned bear is summoning sick.
	exile, back := -1, -1
	for i, ev := range e.L.Events {
		if ev.Kind != events.MoveZone || ev.Obj != bear {
			continue
		}
		if ev.To == state.ZExile && exile < 0 {
			exile = i
		}
		if ev.To == state.ZBattlefield && exile >= 0 && back < 0 {
			back = i
		}
	}
	if exile < 0 || back < 0 || back < exile {
		t.Fatalf("want a MoveZone to exile then to battlefield for the bear; got exile@%d back@%d", exile, back)
	}
	o := e.G.Obj(bear)
	if o == nil || o.Zone != state.ZBattlefield || o.Owner != 0 {
		t.Fatalf("bear not back on seat 0's battlefield: %+v", o)
	}
	if !o.SummonSick {
		t.Fatal("returned bear is not summoning sick: it never re-entered")
	}
	replayCheck(t, e, cfg)
}
