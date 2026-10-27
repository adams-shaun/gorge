package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestDelayedRegistrationRememberedCountsItsCapture pins the delayed-shape
// half of rememberedExcludingCapture: a delayed registration's saved set is
// authored memory, even though trigger_delayed seeds it into Captured as the
// referent for the other readers. TriggerContext.DelayedRemembered is the
// same shape test filterCapturedRemembered applies, so the numeric readers
// must agree with the selection readers on the same ctx.
//
// The control below is the ordinary-trigger exclusion the engine contract
// pins: Captured is the firing event's object (no DelayedRemembered), so a
// Remembered$Amount read of it is 0.
func TestDelayedRegistrationRememberedCountsItsCapture(t *testing.T) {
	h := newHost(t, 2)
	a := h.g.AddObject(mkCard(t, "Name:A\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	b := h.g.AddObject(mkCard(t, "Name:B\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	saved := []state.Target{{Obj: a.ID}, {Obj: b.ID}}

	c := &Ctx{
		Source:     a.ID,
		Controller: 0,
		Remembered: append([]state.Target(nil), saved...),
		Captured:   append([]state.Target(nil), saved...),
		TriggerContext: TriggerContext{
			DelayedRemembered: append([]state.Target(nil), saved...),
		},
	}
	// Precondition: the fixture really has the delayed shape (all three
	// non-empty lists, Captured == DelayedRemembered). A vacuous ctx would
	// pass the count assertion for the wrong reason.
	if a.ID == b.ID {
		t.Fatalf("precondition: fixture objects share an id %d", a.ID)
	}
	if len(c.Remembered) != 2 || len(c.Captured) != 2 || len(c.DelayedRemembered) != 2 {
		t.Fatalf("precondition: lists are not all two long: remembered=%v captured=%v delayed=%v",
			c.Remembered, c.Captured, c.DelayedRemembered)
	}
	if !sameTargets(c.Captured, c.DelayedRemembered) {
		t.Fatalf("precondition: Captured %v != DelayedRemembered %v", c.Captured, c.DelayedRemembered)
	}
	if n, ok := EvalCountOK(h, c, "Remembered$Amount"); !ok || n != 2 {
		t.Fatalf("delayed Remembered$Amount = %d ok=%v, want 2 true", n, ok)
	}

	// Control: an ordinary trigger's fire-time event capture is still
	// excluded, because no DelayedRemembered marks it as authored memory.
	ordinary := &Ctx{
		Source:     a.ID,
		Controller: 0,
		Remembered: []state.Target{{Obj: a.ID}},
		Captured:   []state.Target{{Obj: a.ID}},
	}
	if len(ordinary.Remembered) == 0 || !sameTargets(ordinary.Remembered, ordinary.Captured) {
		t.Fatalf("precondition: ordinary ctx is not a capture-only Remembered==Captured shape: %+v", ordinary)
	}
	if n, ok := EvalCountOK(h, ordinary, "Remembered$Amount"); !ok || n != 0 {
		t.Fatalf("ordinary capture Remembered$Amount = %d ok=%v, want 0 true (event capture must stay excluded)", n, ok)
	}
}
