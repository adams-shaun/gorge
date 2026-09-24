package effects

// api:Phases with an `AllValid$` affected set: 10 corpus lines across 8 files
// carry `DB$ Phases | AllValid$ <filter>` (Out of Time, Unyaro, Disciple of
// Caelus Nin, Galadriel's Dismissal, The City on the Edge of Forever, Teferi's
// Realm, Teferi Timeless Voyager, Time and Tide). Before this test, effPhases
// read Defined$ only, so every one of them silently phased nothing out.

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// phasesOutIDs returns the ids a PhaseOut event on the fake host's log names.
func phasesOutIDs(h *fakeHost) map[state.ObjID]bool {
	out := map[state.ObjID]bool{}
	for _, ev := range h.log {
		if ev.Kind == events.PhaseOut {
			out[ev.Obj] = true
		}
	}
	return out
}

// phasesBattlefieldBoard puts one 1/1 Fixture creature per seat on the
// BATTLEFIELD (fixtureHost's objects start in the library), returning the host
// and a Ctx sourced at seat 0's creature.
func phasesBattlefieldBoard(t *testing.T) (*fakeHost, *Ctx, []state.ObjID) {
	t.Helper()
	h, _ := fixtureHost(t)
	card := mkCard(t, "Name:Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n")
	third := h.g.AddObject(card, 0)
	battle0 := []state.ObjID{}
	battle1 := []state.ObjID{}
	for _, id := range []state.ObjID{1, third.ID} {
		o := h.g.Obj(id)
		o.Zone = state.ZBattlefield
		battle0 = append(battle0, id)
	}
	o2 := h.g.Obj(2)
	o2.Zone = state.ZBattlefield
	battle1 = append(battle1, 2)
	h.g.SetZone(state.ZBattlefield, 0, battle0)
	h.g.SetZone(state.ZBattlefield, 1, battle1)
	return h, &Ctx{Source: 1, Controller: 0}, []state.ObjID{1, 2, third.ID}
}

// TestPhasesAllValidSweepsEveryBattlefieldMatch pins that `DB$ Phases |
// AllValid$ Creature` phases out EVERY battlefield creature, not just the
// source: a phased-in battlefield state is the precondition, and a second
// seat's creature is the control proving the sweep is not self-only.
func TestPhasesAllValidSweepsEveryBattlefieldMatch(t *testing.T) {
	h, ctx, ids := phasesBattlefieldBoard(t)
	for _, id := range ids {
		o := h.g.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: object %d is not a battlefield permanent: %+v", id, o)
		}
	}

	Resolve(h, ctx, sa(t, "SP$ Phases | AllValid$ Creature"))

	got := phasesOutIDs(h)
	for _, id := range ids {
		if !got[id] {
			t.Errorf("AllValid$ Creature did not phase out battlefield creature %d (events %v)", id, got)
		}
	}
	// And nothing off the battlefield was touched.
	if len(got) != 3 {
		t.Fatalf("phased out %d objects, want exactly the 3 battlefield creatures", len(got))
	}
}

// TestPhasesAllValidFilterIsHonoured pins that the AllValid$ SPEC, not just
// its presence, is evaluated: `Creature.YouCtrl` sweeps only the resolving
// controller's creatures.
func TestPhasesAllValidFilterIsHonoured(t *testing.T) {
	h, ctx, _ := phasesBattlefieldBoard(t)
	// fixtureHost: object 1 is seat 0's, object 2 is seat 1's.
	if h.g.Obj(1).Controller != 0 || h.g.Obj(2).Controller != 1 {
		t.Fatalf("precondition: fixtureHost controllers = %d/%d, want 0/1", h.g.Obj(1).Controller, h.g.Obj(2).Controller)
	}

	Resolve(h, ctx, sa(t, "SP$ Phases | AllValid$ Creature.YouCtrl"))

	got := phasesOutIDs(h)
	if !got[1] {
		t.Errorf("Creature.YouCtrl did not phase out seat 0's creature (events %v)", got)
	}
	if got[2] {
		t.Errorf("Creature.YouCtrl phased out the OPPONENT's creature: the AllValid$ spec was not evaluated")
	}
}

// TestPhasesPhaseInOrOutToggles pins Forge's `PhaseInOrOut$` semantics (the
// 8 corpus-carrier shape, Time and Tide's oracle in one pass): an affected
// permanent that is phased out phases IN while one that is phased in phases
// OUT, in the same resolution. Before this fix the parameter was treated as
// an unconditional phase-out, so every `Defined$ Remembered | PhaseInOrOut$
// True` phase-back-in body (Out of Time, Oubliette, Unyaro, Ferris Wheel,
// The Moment, The Pandora, The Doctor's Childhood Barn) phased nothing in.
func TestPhasesPhaseInOrOutToggles(t *testing.T) {
	h, ctx, ids := phasesBattlefieldBoard(t)
	// Phase ids[1] (object 2) out first, so the board has one of each state.
	h.Emit(events.Event{Kind: events.PhaseOut, Obj: ids[1], Amount: 1})
	if !h.g.Obj(ids[1]).PhasedOut || h.g.Obj(ids[0]).PhasedOut {
		t.Fatalf("precondition: phased-out states = %v/%v, want true/false",
			h.g.Obj(ids[1]).PhasedOut, h.g.Obj(ids[0]).PhasedOut)
	}

	Resolve(h, ctx, sa(t, "SP$ Phases | AllValid$ Creature | PhaseInOrOut$ True"))

	// The event stream carries the phase-in for ids[1] and phase-outs for the
	// two phased-in creatures: read the LAST PhaseOut per object, so the
	// pre-phased-out id's earlier event cannot mask the toggle.
	last := map[state.ObjID]int32{}
	for _, ev := range h.log {
		if ev.Kind == events.PhaseOut {
			last[ev.Obj] = ev.Amount
		}
	}
	if got := last[ids[1]]; got != -1 {
		t.Errorf("phased-out creature %d got Amount %d, want -1 (phase IN)", ids[1], got)
	}
	for _, id := range []state.ObjID{ids[0], ids[2]} {
		if got := last[id]; got != 1 {
			t.Errorf("phased-in creature %d got Amount %d, want 1 (phase OUT)", id, got)
		}
	}
	// And the fold agrees: the two states swapped.
	if h.g.Obj(ids[1]).PhasedOut {
		t.Fatalf("creature %d still phased out after a toggle phase-in", ids[1])
	}
	if !h.g.Obj(ids[0]).PhasedOut {
		t.Fatalf("creature %d not phased out after a toggle phase-out", ids[0])
	}
}

// TestPhasesPlainPhaseOutSkipsAlreadyPhasedOut pins Forge's plain-branch
// guard: without PhaseInOrOut$, an already-phased-out permanent gets no
// second PhaseOut event (a no-op fold and a dishonest log line otherwise).
func TestPhasesPlainPhaseOutSkipsAlreadyPhasedOut(t *testing.T) {
	h, ctx, ids := phasesBattlefieldBoard(t)
	h.Emit(events.Event{Kind: events.PhaseOut, Obj: ids[1], Amount: 1})
	before := len(h.log)
	Resolve(h, ctx, sa(t, "SP$ Phases | AllValid$ Creature"))
	var onTwo int
	for _, ev := range h.log[before:] {
		if ev.Kind == events.PhaseOut && ev.Obj == ids[1] {
			onTwo++
		}
	}
	if onTwo != 0 {
		t.Fatalf("plain AllValid$ phase-out re-phased the already-phased-out creature %d (%d events)", ids[1], onTwo)
	}
	// The phased-in ones did phase out, so the skip is not a blanket no-op.
	if !h.g.Obj(ids[0]).PhasedOut {
		t.Fatalf("plain AllValid$ phase-out did not phase out the phased-in creature %d", ids[0])
	}
}

// TestPhasesDefinedStillWinsWithoutAllValid pins that the AllValid$ branch is
// additive, not a replacement: a body with no AllValid$ keeps Defined$'s
// target/source dispatch (the shape every Talon Gates / Guardian of Faith
// carrier uses).
func TestPhasesDefinedStillWinsWithoutAllValid(t *testing.T) {
	h, ctx, _ := phasesBattlefieldBoard(t)
	Resolve(h, ctx, sa(t, "SP$ Phases | Defined$ Self"))
	got := phasesOutIDs(h)
	if !got[1] || len(got) != 1 {
		t.Fatalf("Defined$ Self phased out %v, want only the source object 1", got)
	}
}
