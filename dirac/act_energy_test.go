package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEnergy: OpGainEnergy gives the players named energy (CR 122.1); a
// CostPayEnergy cost is payable with N (costObjPayable, the canActivate /
// pickCostPayable gate) and paying spends it; the unless / if-you-do kind
// (kop.PayEnergy) gates on the same count.
func TestEnergy(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 0)
	sc.s.res.e.ctrl = 0
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	gain := kop.Instr{Op: kop.OpGainEnergy, Obj: kop.RefController, A: kop.Const(2)}
	sc.s.exec(sc.k, &gain)
	if sc.s.Energy(0) != 2 || sc.s.Energy(1) != 0 {
		t.Fatalf("energy %d/%d after gaining 2, want 2/0", sc.s.Energy(0), sc.s.Energy(1))
	}
	each := kop.Instr{Op: kop.OpGainEnergy, Obj: kop.RefEachPlayer, A: kop.Const(1)}
	sc.s.exec(sc.k, &each)
	if sc.s.Energy(0) != 3 || sc.s.Energy(1) != 1 {
		t.Fatalf("energy %d/%d after each player gains 1, want 3/1", sc.s.Energy(0), sc.s.Energy(1))
	}
	c := kop.Cost{Flags: kop.CostPayEnergy, N: 3}
	if !sc.s.pickCostPayable(sc.k, 0, &c, NoObj) || sc.s.pickCostPayable(sc.k, 1, &c, NoObj) {
		t.Fatal("PayEnergy<3>: payable by 3 energy and not by 1")
	}
	if !sc.s.canPayGen(sc.k, 1, kop.PayEnergy, 1, 0, 0) || sc.s.canPayGen(sc.k, 1, kop.PayEnergy, 2, 0, 0) {
		t.Fatal("kop.PayEnergy: 1 energy pays 1, not 2")
	}
	sc.s.payEnergy(0, &c)
	if sc.s.Energy(0) != 0 || sc.s.pickCostPayable(sc.k, 0, &c, NoObj) {
		t.Fatalf("energy %d after paying 3, want 0 and unpayable", sc.s.Energy(0))
	}
	big := kop.Instr{Op: kop.OpGainEnergy, Obj: kop.RefController, A: kop.Const(300)}
	sc.s.exec(sc.k, &big)
	if sc.s.Energy(0) != 255 {
		t.Fatalf("energy %d, want saturated 255", sc.s.Energy(0))
	}
}
