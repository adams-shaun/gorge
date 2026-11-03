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
		if !s.matchCardGen(k, in.F, c, p, s.res.e.ctrl, o) {
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
		if !s.matchCardGen(k, in.F, c, p, s.res.e.ctrl, o) {
			continue
		}
		if g := s.eachGroup(k, in.F, c, p, s.res.e.ctrl, o); g >= 0 && picked&(1<<g) == 0 {
			d.add(MakeAction(AkChoose, uint8(o), 0, 0))
		}
	}
}
