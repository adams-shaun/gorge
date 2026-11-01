package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExileSelfCost: CostExileSelf (gorge Exile<1/CARDNAME>) moves the
// source to exile -- not the graveyard -- as the cost is paid.
func TestExileSelfCost(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 0)
	sc.s.exileSelfCost(sc.k, sc.slot(a))
	if z := sc.s.objs[a].zone; z != kop.ZoneExile {
		t.Fatalf("source zone %d after the exile-self cost, want exile", z)
	}
}
