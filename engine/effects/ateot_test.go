package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// These tests pin scheduleAtEOT's degrade contract at the unit level, where
// the affected set can be forced empty -- the case rules' real-card pins
// cannot reach.

// TestAtEOTOutOfScopeNotesEvenWithNoAffectedObject: an out-of-scope AtEOT$
// value must produce its one loud Note even when the body affected nothing
// (the empty-affected early return must not swallow the note: an empty
// affected set is exactly when a silent drop is easiest to miss).
func TestAtEOTOutOfScopeNotesEvenWithNoAffectedObject(t *testing.T) {
	h := newHost(t, 2)
	// No targets in context: Defined() resolves to nothing, so the pump's
	// affected set is empty.
	Resolve(h, &Ctx{Controller: 0}, sa(t, "DB$ Pump | AtEOT$ Library"))
	notes := 0
	for _, e := range h.log {
		if e.Kind == events.Note && strings.Contains(e.Text, "AtEOT$ Library is not implemented") {
			notes++
		}
	}
	if notes != 1 {
		t.Fatalf("empty affected set produced %d out-of-scope notes, want exactly 1", notes)
	}
	if len(h.g.Delayed) != 0 {
		t.Fatalf("an out-of-scope value registered %d delayed triggers, want 0", len(h.g.Delayed))
	}
}

// TestAtEOTInScopeValueWithNoAffectedObjectIsSilent: the mirror leaf. An
// in-scope value over an empty affected set has nothing to schedule and is
// correctly silent -- no note (nothing was dropped) and no registration.
func TestAtEOTInScopeValueWithNoAffectedObjectIsSilent(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "DB$ Pump | AtEOT$ Exile"))
	for _, e := range h.log {
		if e.Kind == events.Note && strings.Contains(e.Text, "AtEOT$") {
			t.Fatalf("an in-scope value over an empty affected set must be silent; got %q", e.Text)
		}
	}
	if len(h.g.Delayed) != 0 {
		t.Fatalf("an empty affected set registered %d delayed triggers, want 0", len(h.g.Delayed))
	}
}

// TestAtEOTOutOfScopeNoteIsOnePerCall: the per-call contract at unit level --
// two calls of the shared reader emit two notes (the caller owns batching:
// effToken's mint loop collects its mints and calls once per resolution).
func TestAtEOTOutOfScopeNoteIsOnePerCall(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Controller: 0}
	s := sa(t, "DB$ Pump | AtEOT$ Library")
	scheduleAtEOT(h, c, s, nil)
	scheduleAtEOT(h, c, s, nil)
	notes := 0
	for _, e := range h.log {
		if e.Kind == events.Note && strings.Contains(e.Text, "AtEOT$ Library is not implemented") {
			notes++
		}
	}
	if notes != 2 {
		t.Fatalf("two calls produced %d notes, want 2 (one per call)", notes)
	}
}

// TestAtEOTStalePromiseOnABattlefieldObject: an in-scope value over a real
// battlefield object registers one DelayedRegister whose Source is the
// affected object, StepEnd, and the caller's controller -- the wire shape
// events.Apply's DelayedRegister case folds.
func TestAtEOTStalePromiseOnABattlefieldObject(t *testing.T) {
	h, c := fixtureHost(t)
	s := sa(t, "DB$ Pump | AtEOT$ Exile")
	scheduleAtEOT(h, c, s, []state.ObjID{c.Source})
	if len(h.g.Delayed) != 1 {
		t.Fatalf("delayed registrations = %d, want 1", len(h.g.Delayed))
	}
	dt := h.g.Delayed[0]
	if dt.Source != c.Source || dt.Phase != state.StepEnd || dt.Execute != "__kwWarpExile" || dt.Controller != 0 {
		t.Fatalf("registration = %+v, want Source %d / StepEnd / __kwWarpExile / controller 0", dt, c.Source)
	}
}

// TestAtEOTEndOfTurnSpellingSchedulesTheRider: upstream's DB$ EndOfTurn
// (Brackwater Elemental, Momo's Heist) parses to Pump, and that pump's AtEOT$
// Sacrifice registers one delayed sacrifice of the object it names.
func TestAtEOTEndOfTurnSpellingSchedulesTheRider(t *testing.T) {
	h := newHost(t, 2)
	id := putCreature(t, h, 0, "Probe Beast", "2/2", "")
	s := sa(t, "DB$ EndOfTurn | Defined$ Self | AtEOT$ Sacrifice")
	if s.API != "Pump" {
		t.Fatalf("EndOfTurn parsed to %s, want Pump", s.API)
	}
	Resolve(h, &Ctx{Controller: 0, Source: id}, s)
	if len(h.g.Delayed) != 1 {
		t.Fatalf("EndOfTurn AtEOT$ Sacrifice registered %d delayed triggers, want 1", len(h.g.Delayed))
	}
}
