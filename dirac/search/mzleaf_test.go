package search

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestMZLeafMatchesGameStateEvaluator3: the score and value equal
// GameStateEvaluator3's arithmetic worked by hand (life 20 vs 14, hands 3
// vs 5, permanents of mana value 0,0,2 vs 0,3), and on walked positions
// the two seats' values sum to 1 and an even start is 0.5.
func TestMZLeafMatchesGameStateEvaluator3(t *testing.T) {
	mine := mzScore(20, 3, 1+1+3) // 12 + 1.8333 + 5
	theirs := mzScore(14, 5, 1+4) // 8.4 + 2.2833 + 5
	if math.Abs(mine-18.833333) > 1e-5 || math.Abs(theirs-15.683333) > 1e-5 {
		t.Fatalf("scores %.6f %.6f, want 18.833333 15.683333", mine, theirs)
	}
	if v := mzValue(mine, theirs); math.Abs(float64(v)-0.603479) > 1e-5 {
		t.Fatalf("value %.6f, want 0.603479 = (tanh(3.15/15)+1)/2", v)
	}
	if v := mzScore(-3, 0, 0); v != 0 {
		t.Fatalf("negative life scores %v, want 0", v)
	}
	tbl, da, db := landMatchup(t)
	n := 0
	mzxWalk(t, tbl, da, db, 40, func(s *dirac.State, d *dirac.Decision) bool {
		a, b := MZLeaf{}.Value(s, d, 0), MZLeaf{}.Value(s, d, 1)
		if math.Abs(float64(a+b)-1) > 1e-5 || a < 0 || a > 1 {
			t.Fatalf("seat values %v + %v", a, b)
		}
		n++
		return true
	})
	s := dirac.NewConfig(tbl, da, db, 1, dirac.Config{})
	if v := (MZLeaf{}).Value(s, nil, 0); s.HandSize(0) == s.HandSize(1) && v != 0.5 {
		t.Fatalf("even start valued %v", v)
	}
	if n == 0 {
		t.Fatal("no positions walked")
	}
}
