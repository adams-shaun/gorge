package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// drainDiscipleETB answers Disciple of Caelus Nin's ETB asks (each player
// chooses permanents) keeping everything offered, and drains the stack, so
// only the printed CantPhaseIn static is under test.
func drainDiscipleETB(t *testing.T, e *Engine) {
	t.Helper()
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
}

// phaseOutAll phases the permanents out and asserts they really are.
func phaseOutAll(t *testing.T, e *Engine, ids ...state.ObjID) {
	t.Helper()
	for _, id := range ids {
		e.emit(events.Event{Kind: events.PhaseOut, Obj: id, Amount: 1})
	}
	for _, id := range ids {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || !o.PhasedOut {
			t.Fatalf("precondition: %d not a phased-out battlefield permanent (obj %v)", id, o != nil)
		}
	}
}

// assertPhasedInOnce checks each id is phased in with exactly one logged
// phase-in event.
func assertPhasedInOnce(t *testing.T, e *Engine, ids ...state.ObjID) {
	t.Helper()
	for _, id := range ids {
		if o := e.G.Obj(id); o == nil || o.PhasedOut {
			t.Errorf("CR 502.1/702.26a: %d stayed phased out at the untap step (obj %v)", id, o != nil)
		}
		if n := cantPhaseEvents(e, id, false); n != 1 {
			t.Errorf("%d: want exactly one logged phase-in, got %d", id, n)
		}
	}
}

// TestCantPhaseInSourceAmongSimultaneousUntapPhaseIns: a phased-out
// CantPhaseIn source (Disciple of Caelus Nin, earliest in battlefield order)
// is inert while phased out, and the untap step's phase-ins are one
// simultaneous action (CR 502.1, 702.26a): it must not hold back the
// permanents after it when it phases in.
func TestCantPhaseInSourceAmongSimultaneousUntapPhaseIns(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 1601, "Disciple of Caelus Nin", "Grizzly Bears", "Hill Giant")
	disc := moveSeededCard(t, e, 0, cr702corpusCard(t, "Disciple of Caelus Nin"), state.ZBattlefield)
	drainDiscipleETB(t, e)
	bears := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	giant := moveSeededCard(t, e, 0, cr702corpusCard(t, "Hill Giant"), state.ZBattlefield)
	if z := e.G.Zone(state.ZBattlefield, 0); !(indexOfID(z, disc) < indexOfID(z, bears) && indexOfID(z, bears) < indexOfID(z, giant)) {
		t.Fatalf("precondition: battlefield order %v is not disc, bears, giant", z)
	}
	phaseOutAll(t, e, disc, bears, giant)
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	assertPhasedInOnce(t, e, disc, bears, giant)
	replayCheck(t, e, cfg)
}

// TestCantPhaseInSiblingSourcesSimultaneousUntapPhaseIns: two CantPhaseIn
// carriers and ordinary permanents all phased out under one player all phase
// in together.
func TestCantPhaseInSiblingSourcesSimultaneousUntapPhaseIns(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 1602, "Disciple of Caelus Nin", "Disciple of Caelus Nin", "Grizzly Bears", "Grizzly Bears")
	d1 := moveSeededCard(t, e, 0, cr702corpusCard(t, "Disciple of Caelus Nin"), state.ZBattlefield)
	drainDiscipleETB(t, e)
	b1 := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	d2 := moveSeededCard(t, e, 0, cr702corpusCard(t, "Disciple of Caelus Nin"), state.ZBattlefield)
	drainDiscipleETB(t, e)
	b2 := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	ids := []state.ObjID{d1, b1, d2, b2}
	phaseOutAll(t, e, ids...)
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	assertPhasedInOnce(t, e, ids...)
	replayCheck(t, e, cfg)
}

// TestCantPhaseInLiveAtUntapStillBlocks pins the boundary: a Disciple that is
// phased IN when the untap step begins still holds every phased-out permanent
// out, and a blocked candidate logs no phase-in.
func TestCantPhaseInLiveAtUntapStillBlocks(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 1603, "Disciple of Caelus Nin", "Grizzly Bears", "Hill Giant")
	bears := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	giant := moveSeededCard(t, e, 0, cr702corpusCard(t, "Hill Giant"), state.ZBattlefield)
	phaseOutAll(t, e, bears, giant)
	disc := moveSeededCard(t, e, 0, cr702corpusCard(t, "Disciple of Caelus Nin"), state.ZBattlefield)
	drainDiscipleETB(t, e)
	if o := e.G.Obj(disc); o == nil || o.Zone != state.ZBattlefield || o.PhasedOut {
		t.Fatalf("precondition: Disciple not a phased-in battlefield permanent (obj %v)", o != nil)
	}
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	for _, id := range []state.ObjID{bears, giant} {
		if o := e.G.Obj(id); o == nil || !o.PhasedOut {
			t.Errorf("%d phased in despite a live CantPhaseIn (obj %v)", id, o != nil)
		}
		if n := cantPhaseEvents(e, id, false); n != 0 {
			t.Errorf("%d: %d phase-in events for a blocked candidate", id, n)
		}
	}
	replayCheck(t, e, cfg)
}

func indexOfID(z []state.ObjID, id state.ObjID) int {
	for i, x := range z {
		if x == id {
			return i
		}
	}
	return -1
}
