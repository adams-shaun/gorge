package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapAll: OpTapAll (gorge effTapAll) taps every untapped permanent
// matching the filter (Valley Floodcaller's "Otters you control") and no
// other; an already tapped one stays tapped.
func TestTapAll(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a, b, c := sc.perm(fc, 0), sc.perm(fc, 0), sc.perm(fc, 1)
	sc.s.perms[sc.slot(b)].flags |= pfTapped
	sc.s.res.e.ctrl = 0
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	in := kop.Instr{Op: kop.OpTapAll, F: firstInstr(t, sc, fc, kop.OpUntapAll).F}
	opp := in
	opp.Obj = kop.RefOpponent // a target player's battlefield only
	sc.s.exec(sc.k, &opp)
	if sc.tapped(a) {
		t.Error("scoped to the opponent: the controller's Otter was tapped")
	}
	sc.s.exec(sc.k, &in)
	if !sc.tapped(a) || !sc.tapped(b) {
		t.Error("the controller's Otters: want both tapped")
	}
	if sc.tapped(c) {
		t.Error("the opponent's Otter was tapped")
	}
}
