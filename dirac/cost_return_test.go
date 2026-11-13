package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReturnSelfCost: a CostReturnSelf activation (gorge Return<1/CARDNAME>)
// returns its source to its owner's hand as it is paid; the ability still
// resolves.
func TestReturnSelfCost(t *testing.T) {
	sc, id := newVerbScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}},
		func(tbl *Table, id map[string]CardID) {
			fc := id["Valley Floodcaller"]
			tbl.Cards[fc].Abilities = append(tbl.Cards[fc].Abilities,
				kop.Ability{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostReturnSelf},
					Prog: []kop.Instr{{Op: kop.OpGainEnergy, Obj: kop.RefController, A: kop.Const(1)}}})
		})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 0)
	sc.do(t, AkActivate, TCode(a), scAny)
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision kind %d after activating, want priority", d.Kind)
	}
	if sc.s.objs[a].zone != kop.ZoneHand {
		t.Fatalf("the source is in zone %d after paying, want the hand", sc.s.objs[a].zone)
	}
	sc.passBoth(t)
	sc.s.Step()
	if sc.s.Energy(0) != 1 {
		t.Fatalf("energy %d: the ability did not resolve", sc.s.Energy(0))
	}
}

// TestPayReturn: kop.PayReturn (gorge unless returncost) is payable while
// the payer controls a matching permanent (countReturn).
func TestPayReturn(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 0)
	sc.s.res.e.ctrl = 0
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	f := firstInstr(t, sc, fc, kop.OpUntapAll).F // Otters you control
	if !sc.s.canPayGen(sc.k, 0, kop.PayReturn, 1, 0, f) || sc.s.canPayGen(sc.k, 0, kop.PayReturn, 2, 0, f) || sc.s.canPayGen(sc.k, 1, kop.PayReturn, 1, 0, f) {
		t.Fatal("PayReturn: one Otter pays 1, not 2; the opponent controls none")
	}
}

// TestReturnCostPick: a CostReturn activation (gorge Return<N/Spec>,
// returnAsk) poses one ChCostReturn pick over the payer's matching
// permanents (the source included), even with the choice among two, and
// returns the chosen one to its owner's hand as the cost is paid.
func TestReturnCostPick(t *testing.T) {
	sc, id := newVerbScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}},
		func(tbl *Table, id map[string]CardID) {
			fc := id["Valley Floodcaller"]
			var f kop.FilterID
			for _, a := range tbl.Cards[fc].Abilities {
				for _, in := range a.Prog {
					if in.Op == kop.OpUntapAll {
						f = in.F // Otters you control
					}
				}
			}
			tbl.Cards[fc].Abilities = append(tbl.Cards[fc].Abilities,
				kop.Ability{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostReturn, N: 1, Filter: f},
					Prog: []kop.Instr{{Op: kop.OpGainEnergy, Obj: kop.RefController, A: kop.Const(1)}}})
		})
	fc := id["Valley Floodcaller"]
	a, b := sc.perm(fc, 0), sc.perm(fc, 0)
	sc.do(t, AkActivate, TCode(a), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChCostReturn || d.N != 2 {
		t.Fatalf("return pick: kind %d sub %d n %d, want ChCostReturn over both Otters", d.Kind, d.Sub, d.N)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(b), 0, 0))
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision kind %d after the pick, want priority", d.Kind)
	}
	if sc.s.objs[b].zone != kop.ZoneHand || !sc.onBF(a) {
		t.Fatalf("returned: b in zone %d, a on the battlefield %v", sc.s.objs[b].zone, sc.onBF(a))
	}
	sc.passBoth(t)
	sc.s.Step()
	if sc.s.Energy(0) != 1 {
		t.Fatalf("energy %d: the ability did not resolve", sc.s.Energy(0))
	}
}
