package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A cost reduction that tests the spell's mana value reads it with {X} in it
// once X is announced (CR 202.3e, 601.2b, 601.2f): The Scarlet Witch makes an
// instant or sorcery of mana value 4 or more cost {2} less, so {X}{R}{R} cast
// for X = 2 costs {R}{R} and a larger X can be paid by less mana than a
// smaller one. gorge's X ask prices every X up to its ceiling (pool, graveyard
// and convoke credit, and one more) through the cost composed at that X; the
// ask here does the same for the values past the unreduced maximum.

// xHypoCost is the cost of the spell being built as if X were x.
func (s *State) xHypoCost(k *prep, x int) kop.ManaCost {
	b := s.bld()
	ox, fixed := b.e.x, b.fixedGen
	b.e.x = uint8(x)
	s.syncBuild()
	c := s.buildCost(k)
	b.e.x, b.fixedGen = ox, fixed
	s.syncBuild()
	return c
}

// xReducedCeiling is the largest X the ask prices past mx: mx itself unless
// the cost at gorge's ceiling differs from the one at X = 0.
func (s *State) xReducedCeiling(k *prep, e *stackEntry, cost *kop.ManaCost, av *avail, mx int) int {
	if e.kind != seSpell || cost.X == 0 || !s.statM.any() {
		return mx
	}
	ceil := min(av.total+int(s.pl[SeatIx(e.ctrl)].graveN)+s.convokeXCredit(k)+1, maxXAsk)
	if ceil <= mx {
		return mx
	}
	if c := s.xHypoCost(k, ceil); c.Generic == cost.Generic && c.XRed == cost.XRed {
		return mx
	}
	return ceil
}

// xReducedPays reports whether av pays the spell being built at X = x.
func (s *State) xReducedPays(k *prep, av *avail, x int) bool {
	c := s.xHypoCost(k, x)
	return av.affords(&c, pipMaskOf(&c), c.XGeneric(x))
}
