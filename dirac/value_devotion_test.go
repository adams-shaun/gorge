package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDevotion: ExDevotion counts the coloured pips (and hybrid pips once)
// in the printed costs of the controller's permanents (gorge
// devotionCount); the opponent's permanents do not count.
func TestDevotion(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 4}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	sc.perm(fc, 0)
	sc.perm(fc, 0)
	sc.perm(fc, 1)
	sc.perm(id["Plains"], 0)
	cost := sc.k.t.Cards[fc].Cost
	if cost.Pips[1] == 0 {
		t.Fatal("Valley Floodcaller has no blue pip")
	}
	if got, want := sc.s.devotion(sc.k, kop.ColU, 0), 2*int(cost.Pips[1]); got != want {
		t.Fatalf("devotion to blue %d, want %d", got, want)
	}
	if got := sc.s.devotion(sc.k, kop.ColR, 0); got != 2*int(cost.Pips[3]) {
		t.Fatalf("devotion to red %d", got)
	}
	cp := *sc.k.t
	cp.Cards = append([]kop.CardDef(nil), cp.Cards...)
	cp.Cards[fc].Cost.Hyb = [len(kop.ManaCost{}.Hyb)]kop.ColorMask{kop.ColU | kop.ColR, 0}
	sc.k.t = &cp
	if got, want := sc.s.devotion(sc.k, kop.ColU|kop.ColR, 0), 2*(int(cost.Pips[1])+int(cost.Pips[3])+1); got != want {
		t.Fatalf("devotion to blue and red with a U/R hybrid pip %d, want %d", got, want)
	}
}
