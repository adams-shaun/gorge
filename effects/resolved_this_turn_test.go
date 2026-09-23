package effects

import "testing"

// TestResolvedThisTurnCountHead pins the Count$ResolvedThisTurn head the
// Sephiroth, Fabled SOLDIER feedback (fb-20260923T005857Z) turned on: the
// head was unmodelled, so EvalCountOK returned (0, false) and the SVar gate
// over it failed OPEN -- SetState's ConditionCheckSVar$ X transformed the
// creature on the FIRST resolution instead of the fourth. The head now reads
// the tally rules binds onto Ctx, and an unbound Ctx is a modelled zero (the
// gate fails CLOSED), not an unresolvable body.
func TestResolvedThisTurnCountHead(t *testing.T) {
	h := newHost(t, 2)
	// The bound read: the fourth resolution of the turn.
	bound := &Ctx{Controller: 0, ResolvedThisTurn: 4}
	if got, ok := EvalCountOK(h, bound, "Count$ResolvedThisTurn"); !ok || got != 4 {
		t.Fatalf("EvalCountOK(bound) = (%d, %v), want (4, true)", got, ok)
	}
	// Precondition: the assertion is not satisfied by a constant -- a
	// different bound value must read differently, or `got != 4` could pass
	// for a head that always answers 4.
	other := &Ctx{Controller: 0, ResolvedThisTurn: 9}
	if got, ok := EvalCountOK(h, other, "Count$ResolvedThisTurn"); !ok || got != 9 {
		t.Fatalf("EvalCountOK(9) = (%d, %v), want (9, true)", got, ok)
	}
	// An unbound Ctx is a LEGITIMATE ZERO that is still evaluated, so the
	// gate over it fails closed at 0 rather than running its sub anyway.
	unbound := &Ctx{Controller: 0}
	if got, ok := EvalCountOK(h, unbound, "Count$ResolvedThisTurn"); !ok || got != 0 {
		t.Fatalf("EvalCountOK(unbound) = (%d, %v), want (0, true)", got, ok)
	}
}
