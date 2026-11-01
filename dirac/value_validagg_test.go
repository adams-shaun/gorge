package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestValidAgg: ExValidAgg folds gorge's Count$Valid $<prop> suffixes over
// the battlefield matches: extremes (0 with no match), sums, the distinct
// power set and the colours among the matches.
func TestValidAgg(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Seedglaive Mentor", Count: 1}, {Name: "Plains", Count: 8}})
	vf, sw := id["Valley Floodcaller"], id["Seedglaive Mentor"]
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	agg := func(n int16) int {
		x := kop.Expr{Kind: kop.ExValidAgg, N: n, F: f}
		return sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
	}
	if agg(kop.AggGreatestPower) != 0 || agg(kop.AggColors) != 0 || agg(kop.AggSumMV) != 0 {
		t.Fatal("no match must read 0")
	}
	sc.perm(vf, 0)
	sc.perm(vf, 0)
	sc.perm(sw, 0)
	sc.perm(sw, 1) // the opponent's does not match YouCtrl
	cv, cs := sc.k.t.Cards[vf], sc.k.t.Cards[sw]
	mv := func(c kop.CardDef) int { return int(c.Cost.MV()) }
	if got, want := agg(kop.AggGreatestPower), max(int(cv.Power), int(cs.Power)); got != want {
		t.Fatalf("greatest power %d, want %d", got, want)
	}
	if got, want := agg(kop.AggGreatestToughness), max(int(cv.Toughness), int(cs.Toughness)); got != want {
		t.Fatalf("greatest toughness %d, want %d", got, want)
	}
	if got, want := agg(kop.AggGreatestMV), max(mv(cv), mv(cs)); got != want {
		t.Fatalf("greatest mv %d, want %d", got, want)
	}
	if got, want := agg(kop.AggSumToughness), 2*int(cv.Toughness)+int(cs.Toughness); got != want {
		t.Fatalf("total toughness %d, want %d", got, want)
	}
	if got, want := agg(kop.AggSumMV), 2*mv(cv)+mv(cs); got != want {
		t.Fatalf("total mv %d, want %d", got, want)
	}
	wantD := 2
	if cv.Power == cs.Power {
		wantD = 1
	}
	if got := agg(kop.AggDistinctPower); got != wantD {
		t.Fatalf("distinct powers %d, want %d", got, wantD)
	}
	wantC := 0
	for c := cv.Colors | cs.Colors; c != 0; c &= c - 1 {
		wantC++
	}
	if got := agg(kop.AggColors); got != wantC {
		t.Fatalf("colors %d, want %d", got, wantC)
	}
}
