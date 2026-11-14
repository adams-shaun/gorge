package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An announced-X counter removal (Chandra, Hope's Beacon; CR 606.4, 107.3):
// SubCounter<X/LOYALTY> on a planeswalker and SubCounter<X/KIND> on any
// permanent are CostLoyaltyX. The activation announces X 0..the counters of
// Cost.Ctr the source has (ChX, mode = X; gorge's shared xAsk,
// cast_subcounter.go's announced SubCounter bound), then removes that many
// with the other costs (payAbilityCosts). The body reads the announcement as
// Count$xPaid (the stack entry's X). Cold code: the build reaches it only
// behind genPrep.anyLoyaltyX.

// prepLoyaltyX sets anyLoyaltyX for every announced-X counter cost.
func prepLoyaltyX(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if t.Cards[i].Abilities[j].Cost.Flags&kop.CostLoyaltyX != 0 {
				g.anyLoyaltyX = true
			}
		}
	}
}

// loyaltyXOptions poses the X ask of an announced-X counter ability being
// built; false when the ability has no such cost.
//
//go:noinline
func (s *State) loyaltyXOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.Cost.Flags&kop.CostLoyaltyX == 0 {
		return false
	}
	have := 0
	if sl := s.srcSlot(); sl != NoSlot {
		have = s.perms[sl].ctrLow(int(a.Cost.Ctr))
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChX, e.ctrl
	for x := 0; x <= have; x++ {
		// CR 601.2c: an X the body's target bounds read ("untap X target
		// artifacts", Dyfed, the Guiding Hand) is offered only while the
		// target ask could answer it, the same filter the mana X ask applies
		// (gorge xAsk's targetsAnnounceableAt, cast_subcounter.go; xAnswerable
		// for the shared ask). X = 0 is always answerable, so the ask never
		// comes up empty.
		if s.xAnswerable(k, a, e, x) {
			d.add(MakeAction(AkChoose, 0, 0, uint8(x)))
		}
	}
	s.bld().stage = bsX | 0x80 // awaiting X
	return true
}

// counterXClamp min-clamps an X ask's mana bounds by the counters of the
// announced cost's kind the source has (gorge's shared xAsk caps the one X by
// every announced part); unchanged when the ability has no such cost.
//
//go:noinline
func (s *State) counterXClamp(k *prep, mx, hi int) (int, int) {
	e := &s.bld().e
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.Cost.Flags&kop.CostLoyaltyX == 0 {
		return mx, hi
	}
	have := 0
	if sl := s.srcSlot(); sl != NoSlot {
		have = s.perms[sl].ctrLow(int(a.Cost.Ctr))
	}
	return min(mx, have), min(hi, have)
}

// payCounterX removes the announced X counters of kind (the cost's
// CounterChange; CR 606.4 for loyalty, 118.3 for any other kind).
//
//go:noinline
func (s *State) payCounterX(sl Slot, kind int, x uint8) {
	s.perms[sl].seeCtr(s)
	s.perms[sl].subCtrN(s, kind, int(x))
}
