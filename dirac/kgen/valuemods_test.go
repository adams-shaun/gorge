package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDiscoverTailRefused pins the refusal for a Discover
// whose free cast has a board-affecting tail. The kernel queues the free cast
// and runs the tail before it (kernel-discover-freecast-order), so Hit the
// Mother Lode (Discover + DB$ Token) is refused; the standard Discover tail
// (hand + clear) is order-independent and stays accepted.
func TestDiscoverTailRefused(t *testing.T) {
	standard := []lInstr{
		{Op: "Discover", Obj: lRef{Kind: rYou}},
		{Op: "CastFree", Obj: lRef{Kind: rRemembered}, C: zExile},
		{Op: "Discover", C: 1},
		{Op: "ClearRemembered"},
	}
	if kvDiscoverTailRefused(standard) {
		t.Errorf("the standard Discover tail (hand + clear) is refused; want accepted")
	}
	hit := []lInstr{
		{Op: "Discover", Obj: lRef{Kind: rYou}},
		{Op: "CastFree", Obj: lRef{Kind: rRemembered}, C: zExile},
		{Op: "Discover", C: 1},
		{Op: "SkipUnless", C: 1},
		{Op: "CreateToken"},
		{Op: "ClearRemembered"},
	}
	if !kvDiscoverTailRefused(hit) {
		t.Errorf("Hit the Mother Lode's Discover + Token tail is accepted; want a refusal")
	}
	// A CastFree without the Discover lead-in (freeplay, cascade) is not this
	// shape and must keep lowering.
	other := []lInstr{
		{Op: "CastFree", Obj: lRef{Kind: rRemembered}, C: zExile},
		{Op: "CreateToken"},
	}
	if kvDiscoverTailRefused(other) {
		t.Errorf("a non-Discover CastFree with a tail is refused; want accepted")
	}
}

// TestConstantManaValueCompare pins the constant mana-value compare:
// < N lowers to CMCLE N-1 and > N to CMCGE N+1 (mana values are integers).
func TestConstantManaValueCompare(t *testing.T) {
	cb := &cardBuilder{}
	known := func(string) bool { return false }
	for _, tc := range []struct {
		expr string
		pred kop.Pred
		n    int16
	}{
		{"Card.cmcLT10", kop.PredCMCLE, 9},
		{"Card.cmcGT5", kop.PredCMCGE, 6},
		{"Card.cmcLE4", kop.PredCMCLE, 4},
		{"Card.cmcGE7", kop.PredCMCGE, 7},
		{"Card.cmcEQ3", kop.PredCMCEQ, 3},
	} {
		f, err := parseFilter(tc.expr, known, known)
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		kf, err := cb.clause(f.Clauses[0], kop.ZoneBattlefield)
		if err != nil {
			t.Errorf("%s: %v", tc.expr, err)
			continue
		}
		if kf.Pred != tc.pred || kf.PredN != tc.n {
			t.Errorf("%s: pred %v N %d, want %v N %d", tc.expr, kf.Pred, kf.PredN, tc.pred, tc.n)
		}
	}
}
