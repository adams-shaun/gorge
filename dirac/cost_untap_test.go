package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUntapCost: {Q} (kop.CostUntapSelf) is payable only while the source
// is tapped and not summoning sick; paying it untaps it, or removes a stun
// counter instead.
func TestUntapCost(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	a := sc.perm(id["Valley Floodcaller"], 0)
	sl := sc.slot(a)
	p := &sc.s.perms[sl]
	p.flags &^= pfSick
	if sc.s.untapCostOK(sc.k, sl) {
		t.Fatal("payable untapped")
	}
	p.flags |= pfTapped | pfSick
	if sc.s.untapCostOK(sc.k, sl) {
		t.Fatal("payable while summoning sick")
	}
	p.flags &^= pfSick
	if !sc.s.untapCostOK(sc.k, sl) {
		t.Fatal("not payable tapped")
	}
	sc.s.untapCost(sc.k, sl)
	if sc.tapped(a) {
		t.Fatal("still tapped after paying {Q}")
	}
	p.flags |= pfTapped
	p.setCtrN(sc.s, int(kop.CtrStun), 1)
	sc.s.untapCost(sc.k, sl)
	if !sc.tapped(a) || p.ctrLow(int(kop.CtrStun)) != 0 {
		t.Fatal("a stun counter is removed instead")
	}
}
