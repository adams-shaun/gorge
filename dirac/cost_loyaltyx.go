package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A planeswalker's [-X] (Chandra, Hope's Beacon; CR 606.4):
// SubCounter<X/LOYALTY> is CostLoyaltyX. The activation announces X
// 0..the loyalty the walker has (ChX, mode = X; gorge's shared xAsk,
// cast_subcounter.go's announced SubCounter bound), then removes that many
// loyalty counters with the other costs (payAbilityCosts). The body reads
// the announcement as Count$xPaid (the stack entry's X). Cold code: the
// build reaches it only behind genPrep.anyLoyaltyX.

// prepLoyaltyX sets anyLoyaltyX.
func prepLoyaltyX(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if t.Cards[i].Abilities[j].Cost.Flags&kop.CostLoyaltyX != 0 {
				g.anyLoyaltyX = true
			}
		}
	}
}

// loyaltyXOptions poses the X ask of a [-X] ability being built; false when
// the ability has no such cost.
//
//go:noinline
func (s *State) loyaltyXOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if k.t.Cards[e.card].Abilities[e.ab].Cost.Flags&kop.CostLoyaltyX == 0 {
		return false
	}
	have := 0
	if sl := s.srcSlot(); sl != NoSlot {
		have = s.perms[sl].ctrLow(int(kop.CtrLoyalty))
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChX, e.ctrl
	for x := 0; x <= have; x++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(x)))
	}
	s.bld().stage = bsX | 0x80 // awaiting X
	return true
}

// payLoyaltyX removes the announced X loyalty counters (the cost's
// CounterChange, CR 606.4).
//
//go:noinline
func (s *State) payLoyaltyX(sl Slot, x uint8) {
	s.perms[sl].seeCtr(s)
	s.perms[sl].subCtrN(s, int(kop.CtrLoyalty), int(x))
}
