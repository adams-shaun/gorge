package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// coffinQueenFixture: real corpus Coffin Queen to seat 0's battlefield, real
// corpus Grizzly Bears to seat 0's graveyard, SummonSick cleared (raw setup,
// afflict_test.go shape), {2}{B} funded as "BBB" (addMana's digit handling).
func coffinQueenFixture(t *testing.T) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, _ := searchEngine(t, reg, "Coffin Queen")
	queen := searchMoveByName(t, e, "Coffin Queen", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZGraveyard)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: bear zone = %v, want graveyard", o)
	}
	e.G.Obj(queen).SummonSick = false
	addMana(t, e, 0, "BBB")
	return e, queen, bear
}

func coffinQueenAnimate(t *testing.T, e *Engine, queen, bear state.ObjID) {
	t.Helper()
	ab := -1
	for i, sa := range e.G.Obj(queen).Face().Abilities {
		if sa.Kind == "AB" && sa.API == "ChangeZone" {
			ab = i
		}
	}
	if ab < 0 {
		t.Fatal("precondition: Coffin Queen carries no ChangeZone activated ability")
	}
	submitChoices(t, e, abilityOption(t, e, queen, ab).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after activating Coffin Queen: %+v, want the graveyard target ask", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == bear {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("precondition: the graveyard bear is not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the animated bear zone = %v, want battlefield", o)
	}
	if got := e.G.Obj(queen).Remembered; len(got) != 1 || got[0].Obj != bear {
		t.Fatalf("precondition: queen Remembered = %+v, want [%d]", got, bear)
	}
	if inc := e.G.Obj(queen).Incarnation; inc != 1 {
		t.Fatalf("precondition: queen Incarnation = %d, want 1", inc)
	}
}

func coffinQueenLeaveAndStack(t *testing.T, e *Engine, queen, bear state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.MoveZone, Obj: queen, From: state.ZBattlefield, To: state.ZGraveyard})
	stacked := false
	for i := 0; i < 12; i++ {
		if len(e.G.Stack) > 0 {
			stacked = true
			break
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("while waiting for the leave trigger: %+v", d)
		}
		passPriorityOnce(t, e)
	}
	if !stacked {
		t.Fatal("the leave trigger never stacked: CheckSVar$ X (Remembered$Amount) did not read the Queen's remembered list")
	}
	if z := e.G.Obj(queen).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: queen zone = %v, want graveyard", z)
	}
	if inc := e.G.Obj(queen).Incarnation; inc != 2 {
		t.Fatalf("precondition: queen Incarnation = %d, want 2", inc)
	}
	if got := e.G.Obj(queen).Remembered; len(got) != 1 || got[0].Obj != bear {
		t.Fatalf("precondition: the departure keeps Remembered = %+v, want [%d]", got, bear)
	}
}

func TestCoffinQueenLeaveTriggerExilesRemembered(t *testing.T) {
	e, queen, bear := coffinQueenFixture(t)
	coffinQueenAnimate(t, e, queen, bear)
	coffinQueenLeaveAndStack(t, e, queen, bear)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZExile {
		t.Fatalf("bear = %+v, want exile (the leave trigger must exile the creature the Queen animated)", o)
	}
}

func TestCoffinQueenPendingLeaveTriggerExilesRememberedAfterReentry(t *testing.T) {
	e, queen, bear := coffinQueenFixture(t)
	coffinQueenAnimate(t, e, queen, bear)
	coffinQueenLeaveAndStack(t, e, queen, bear)
	e.emit(events.Event{Kind: events.MoveZone, Obj: queen, From: state.ZGraveyard, To: state.ZBattlefield})
	if inc := e.G.Obj(queen).Incarnation; inc != 3 {
		t.Fatalf("precondition: queen Incarnation = %d, want 3 after re-entry", inc)
	}
	if got := e.G.Obj(queen).Remembered; len(got) != 0 {
		t.Fatalf("precondition: the re-entry is a new object: Remembered = %+v, want empty", got)
	}
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZExile {
		t.Fatalf("bear = %+v, want exile: the pending trigger belongs to the old incarnation (CR 400.7)", o)
	}
}
