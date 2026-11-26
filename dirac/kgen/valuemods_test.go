package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

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
