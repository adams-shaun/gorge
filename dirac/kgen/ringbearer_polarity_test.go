package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRingbearerCompareKeepsPolarity: IsRingbearer is a designation word with
// a polarity (CmpN 1 positive, 0 negated), unlike the bare compare words it
// used to share. build.go's clause must carry CmpN into the filter's PredN;
// the kernel reads PredN as the polarity (predPermCold/predCardCold), so
// dropping it inverts the positive word.
func TestRingbearerCompareKeepsPolarity(t *testing.T) {
	known := func(string) bool { return false }
	for _, tc := range []struct {
		expr string
		want int16
	}{
		{"Card.IsRingbearer", 1},
		{"Card.!IsRingbearer", 0},
	} {
		f, err := parseFilter(tc.expr, known, known)
		if err != nil || len(f.Clauses) != 1 {
			t.Fatalf("%s: parsed %v, %v; want one clause", tc.expr, f, err)
		}
		cb := &cardBuilder{}
		kf, err := cb.clause(f.Clauses[0], kop.ZoneBattlefield)
		if err != nil {
			t.Fatalf("%s: clause: %v", tc.expr, err)
		}
		if kf.Pred != kop.PredRingBearer || kf.PredN != tc.want {
			t.Errorf("%s: lowered to Pred %v PredN %d, want PredRingBearer PredN %d", tc.expr, kf.Pred, kf.PredN, tc.want)
		}
	}
}
