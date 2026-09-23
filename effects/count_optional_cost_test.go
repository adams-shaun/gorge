// count_optional_cost_test.go — the Count$OptionalGenericCostPaid.<paid>.
// <unpaid> head at the evaluator level: both branches resolve, the verdict is
// "evaluated" (a CheckSVar gate over it must take the real verdict, not the
// fail-open default), and a DIFFERENT object bound as the count's source —
// what a copy of the cast spell is — reads its own unpaid provenance, never
// the original's paid bit (state.Object.OptionalCostPaid is per-object cast
// provenance; events.Apply's StackCopy does not copy it).
package effects

import (
	"testing"
)

// TestOptionalGenericCostPaidBothBranches pins the dotted branch head: paid
// provenance answers <paid>, unpaid answers <unpaid>, and the head reports
// EVALUATED either way so a CheckSVar gate over it takes the real verdict.
func TestOptionalGenericCostPaidBothBranches(t *testing.T) {
	h, c := fixtureHost(t)
	src := h.Game().Obj(c.Source)
	if n, ok := EvalCountOK(h, c, "Count$OptionalGenericCostPaid.4.2"); !ok || n != 2 {
		t.Fatalf("unpaid = (%d, %v), want (2, true)", n, ok)
	}
	src.OptionalCostPaid = true
	if n, ok := EvalCountOK(h, c, "Count$OptionalGenericCostPaid.4.2"); !ok || n != 4 {
		t.Fatalf("paid = (%d, %v), want (4, true)", n, ok)
	}
	if n := EvalCount(h, c, "Count$OptionalGenericCostPaid.1.0"); n != 1 {
		t.Fatalf("paid .1.0 branch = %d, want 1", n)
	}
	// A malformed body (no dotted branches) fails UNRESOLVABLE, so a gate
	// over it fails its caller's documented direction rather than reading 0.
	if _, ok := EvalCountOK(h, c, "Count$OptionalGenericCostPaid"); ok {
		t.Fatal("branchless body reported evaluated")
	}
}

// TestOptionalGenericCostPaidCastSAIndirection pins the CastSA> route
// (Graven Archfiend's ETB gate "CastSA>Count$OptionalGenericCostPaid.1.0"):
// the indirection binds the count's source to the CastSA object and the
// gate's count reads THAT object's provenance.
func TestOptionalGenericCostPaidCastSAIndirection(t *testing.T) {
	h, c := fixtureHost(t)
	src := h.Game().Obj(c.Source)
	if n, ok := EvalCountOK(h, c, "CastSA>Count$OptionalGenericCostPaid.1.0"); !ok || n != 0 {
		t.Fatalf("unpaid CastSA> = (%d, %v), want (0, true)", n, ok)
	}
	src.OptionalCostPaid = true
	if n, ok := EvalCountOK(h, c, "CastSA>Count$OptionalGenericCostPaid.1.0"); !ok || n != 1 {
		t.Fatalf("paid CastSA> = (%d, %v), want (1, true)", n, ok)
	}
}

// TestOptionalGenericCostPaidCopyDoesNotInherit pins the non-inheritance: a
// SECOND object bound as the count's source — the object identity a stack
// copy of the spell gets — reads its own unpaid provenance while the
// original stays paid.
func TestOptionalGenericCostPaidCopyDoesNotInherit(t *testing.T) {
	h, c := fixtureHost(t)
	src := h.Game().Obj(c.Source)
	src.OptionalCostPaid = true
	if n := EvalCount(h, c, "Count$OptionalGenericCostPaid.4.2"); n != 4 {
		t.Fatalf("original = %d, want 4", n)
	}
	// The fixture's second object (id 2, seat 1's) stands in for the copy: a
	// distinct object id, same face, never paid.
	c2 := &Ctx{Source: 2, Controller: 1}
	if n, ok := EvalCountOK(h, c2, "Count$OptionalGenericCostPaid.4.2"); !ok || n != 2 {
		t.Fatalf("copy = (%d, %v), want (2, true)", n, ok)
	}
}
