package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// EACH A & B picks (gorge eachAlternatives, EachTypeGroups): a search or hand
// pick of one card per listed type. The filter's Or chain is the list, its
// head is marked Filter.Each, and a card belongs to the group of the first
// chain element that matches it, so a card matching two listed types is
// offered once and picking it never blocks the other type's pick. Each group
// is picked from at most once, and the pick count is the number of groups
// that hold a candidate. Cold and out of line: the pick sites test one flag.

// eachOp reports a pick whose filter is an EACH list.
func eachOp(k *prep, in *kop.Instr) bool {
	return in.F != 0 && k.t.Filters[in.F].Each
}

// eachGroup is the index in f's Or chain of the first element matching the
// card, or -1.
//
//go:noinline
func (s *State) eachGroup(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj) int {
	for i := 0; f != 0; i++ {
		if s.matchCardOneAt(k, &k.t.Filters[f], c, owner, ctrl, o, nil) {
			return i
		}
		f = k.t.Filters[f].Or
	}
	return -1
}

// eachPool is the list of cards the EACH pick in progress chooses among: the
// searching player's library, or their hand.
func (s *State) eachPool(k *prep) []Obj {
	pl := &s.pl[s.g.pickP]
	if s.searchZone(k) == kop.ZoneHand {
		return pl.hand[:pl.handN]
	}
	return pl.lib[pl.libLo:pl.libHi]
}

// clampEach sets the pick count to the number of groups with a candidate
// (gorge eachStructuredOptions' ceiling, one pick per type).
//
//go:noinline
func (s *State) clampEach(k *prep, in *kop.Instr) {
	p := s.g.pickP
	var have uint16
	for _, o := range s.eachPool(k) {
		c := s.objs[o].card
		if s.tokenLeftBattlefield(o) || !s.matchCardGen(k, in.F, c, p, s.res.e.ctrl, o) { // a token off the battlefield is no card (CR 111.8)
			continue
		}
		if g := s.eachGroup(k, in.F, c, p, s.res.e.ctrl, o); g >= 0 {
			have |= 1 << g
		}
	}
	s.g.pickLeft = uint8(bits.OnesCount16(have))
}

// eachCands adds the candidates of every group not yet picked from.
//
//go:noinline
func (s *State) eachCands(k *prep, in *kop.Instr, d *Decision, list []Obj) {
	p := s.g.pickP
	var picked uint16
	for i := 0; i < int(s.g.nlook); i++ {
		o := s.resArm().look[i]
		if g := s.eachGroup(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o); g >= 0 {
			picked |= 1 << g
		}
	}
	for _, o := range list {
		if s.picksHas(o) {
			continue
		}
		c := s.objs[o].card
		if s.tokenLeftBattlefield(o) || !s.matchCardGen(k, in.F, c, p, s.res.e.ctrl, o) { // a token off the battlefield is no card (CR 111.8)
			continue
		}
		if g := s.eachGroup(k, in.F, c, p, s.res.e.ctrl, o); g >= 0 && picked&(1<<g) == 0 {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
}

// pickFits reports whether o can join the picks in progress under the
// pick-set constraints a filter head carries (kop.Filter.PickSumMax and
// PickDistinctPower, set by kgen from WithTotalCMC$ and
// WithDifferentPowers$): the picked cards' printed mana values plus o's
// stay within the budget, and o's printed power differs from every picked
// card's. dig: the picks are the dig's lookSel bits over resArm().look;
// otherwise they are resArm().look[:nlook] (a search or ChooseMove's taken
// cards). A filter with neither constraint is the common case and returns
// at once.
//
//go:noinline
func (s *State) pickFits(k *prep, f kop.FilterID, o Obj, dig bool) bool {
	fl := &k.t.Filters[f]
	budget, distinct := fl.PickSumMax, fl.PickDistinctPower
	needSum := budget.K == kop.VConst && budget.N > 0
	if !needSum && !distinct {
		return true
	}
	cand := &k.t.Cards[s.objs[o].card]
	left := int(budget.N) - int(cand.Cost.MV())
	if needSum && left < 0 {
		return false
	}
	var pow int8
	if distinct {
		s.offZonePTRead(k, s.objs[o].card, s.objs[o].owner(), o) // static_offzonept.go
		pow = cand.Power
	}
	for i := 0; i < int(s.g.nlook); i++ {
		if dig && s.g.lookSel&(1<<i) == 0 {
			continue
		}
		q := &k.t.Cards[s.objs[s.resArm().look[i]].card]
		if needSum {
			if left -= int(q.Cost.MV()); left < 0 {
				return false
			}
		}
		if distinct && q.Power == pow {
			return false
		}
	}
	return true
}

// digBoundOpen reports a may-stop Dig whose pick filter carries a pick-set
// bound: gorge poses its take as one multi-select of Max = min(ChangeNum,
// the cards that match and fit the budget on their own), so a pick that
// leaves nothing fitting the running sum or the taken powers still poses
// the Done-only decision until Max is reached (digBoundMax caps pickLeft
// to that Max when the ask is posed).
func (s *State) digBoundOpen(k *prep) bool {
	fl := &k.t.Filters[s.prog(k)[s.res.pc].F]
	b := fl.PickSumMax
	return (b.K == kop.VConst && b.N > 0 || fl.PickDistinctPower) && s.digMayStop(k)
}

// digBoundMax caps a bounded may-stop Dig's pickLeft to gorge's Max: the
// looked-at cards that match the filter and whose own mana value fits the
// budget (the distinct-power bound narrows no card on its own). It runs
// before the first pick, while no card is selected.
func (s *State) digBoundMax(k *prep) {
	if !s.digBoundOpen(k) {
		return
	}
	f := s.prog(k)[s.res.pc].F
	n := 0
	for i := 0; i < int(s.g.nlook); i++ {
		o := s.resArm().look[i]
		if s.g.lookSel&(1<<i) == 0 && s.matchCardGen(k, f, s.objs[o].card, s.objs[o].owner(), s.res.e.ctrl, o) && s.pickFits(k, f, o, true) {
			n++
		}
	}
	if n < int(s.g.pickLeft) {
		s.g.pickLeft = uint8(n)
	}
}
