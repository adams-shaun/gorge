package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Life-only mana abilities (kop.GapLifeManaAb: Treasonous Ogre's "Pay 3 life:
// Add {R}"). gorge's payment planner has no unit for the ability (the cost is
// life with no {T}, so it is neither a census source nor a last-resort tap),
// and the census misses it: a potential play the planner proves insufficient
// gets gorge's one prefix search (rules scriptSearch.visit), which activates
// the source and pays the life. The kernel models it as a census gap whose
// activation pays the life and floats the mana (AvailScript's gaps walk), and
// beside a paid filter as an otherLife step of the filter search
// (mana_mixgaps.go), so the life funds the filter's fee and the filter's mana
// pays the play.
//
// The source neither taps nor needs to be unsick: paying life is not a {T}
// cost, so its mask is not gapMask's (a tapped or summoning-sick source stays
// usable). The life total is not part of the filter search's state key, so
// every activation checks the payer can pay (lifeGapAt; lifeGapMask at the
// root).

// useLifeGap: some card's gapOther mana ability is a life-only one
// (kop.GapLifeManaAb). prepCensusGap sets it, as it is the registered ability
// that counts, not any ability of the table, so its detector holds for none
// (a use bit, not a genPrep field: the fixed State layout is pinned).
var useLifeGap = newGenUse(func(*kop.Table) bool { return false })

// lifeGapSource returns sl's life-only census ability whatever its cost is
// payable: the shape test the filter search uses to route the source to its
// last-resort pass.
func (s *State) lifeGapSource(k *prep, sl Slot) (*kop.Ability, bool) {
	if k.gen == nil || !k.gen.uses.has(useLifeGap) {
		return nil, false
	}
	c := s.cardAt(sl)
	if !k.gen.has(c, gfGapOther) {
		return nil, false
	}
	a := &k.t.Cards[c].Abilities[k.gen.gapOther(c)]
	if !kop.GapLifeManaAb(a) {
		return nil, false
	}
	return a, true
}

// lifeGapAt returns the life-only mana ability of sl when sl is a census gap
// of p with a payable life cost. The life check is per node: the life total is
// not part of the filter search's state key.
func (s *State) lifeGapAt(k *prep, p uint8, sl Slot) (*kop.Ability, bool) {
	a, ok := s.lifeGapSource(k, sl)
	if !ok || int(s.pl[SeatIx(p)].life) < int(a.Cost.N) {
		return nil, false
	}
	return a, true
}

// isLifeGapAt reports whether sl is a payable life-only census gap of p.
func (s *State) isLifeGapAt(k *prep, p uint8, sl Slot) bool {
	_, ok := s.lifeGapAt(k, p, sl)
	return ok
}

// lifeGapMask: p's life-only census gaps whose life cost is payable now. The
// source stays usable tapped and summoning-sick (only a lost ability or a
// can't-be-activated static takes it out), unlike gapMask's {T} sources.
func (s *State) lifeGapMask(k *prep, p uint8) slotMask {
	if k.gen == nil || !k.gen.uses.has(useLifeGap) {
		return slotMask{}
	}
	var m slotMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if s.genLost(k, sl) || k.gen.anyCantAct && s.actForbidden(k, sl, true) {
			continue
		}
		if s.isLifeGapAt(k, p, sl) {
			m = m.with(sl)
		}
	}
	return m
}

// lifeGapAnswers is the answers of sl's activation ask in the script search: a
// fixed production asks nothing (one answer), one of any colour asks which
// (W U B R G).
func (s *State) lifeGapAnswers(k *prep, sl Slot) uint8 {
	a := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.gapOther(s.cardAt(sl))]
	if a.ManaColors == kop.ColAny {
		return 5
	}
	return 1
}

// lifeGapDo activates life gap sl for p with answer ans: pay the life, then
// add the mana. The source is not tapped (a life cost is no {T} cost) and no
// use is spent: the ability is repeatable while the payer has the life.
func (s *State) lifeGapDo(k *prep, p uint8, sl Slot, ans int) {
	c := s.cardAt(sl)
	a := &k.t.Cards[c].Abilities[k.gen.gapOther(c)]
	s.payLifeCost(k, p, &a.Cost, 0)
	col := bits.TrailingZeros8(uint8(a.ManaColors))
	if a.ManaColors == kop.ColAny {
		col = ans
	}
	s.pl[SeatIx(p)].pool[col] += max(a.ManaN, 1)
	s.artTag(k, p, sl, col)
}

// lifeGap branches life gap sl at a priority node of the filter search: its
// colour ask when it makes one of any colour (gorge's scriptOrderAnswers puts
// the play's announced colours first), its one answer otherwise. The life
// check is the node's (lifeGapAt): a path that already paid the life may have
// left too little for another activation.
func (fs *filtSearch) lifeGap(k *prep, t *State, sl Slot, funded int, try func(otherStep) bool) bool {
	if !t.isLifeGapAt(k, fs.p, sl) {
		return false
	}
	n := t.lifeGapAnswers(k, sl)
	if n > 1 && !fs.tick() {
		return false
	}
	if n == 1 {
		return try(otherStep{kind: otherLife})
	}
	var order [5]uint8
	for i := range order {
		order[i] = uint8(i)
	}
	needFirst(order[:], fs.need) // gorge's scriptOrderAnswers on the colour ask
	for _, c := range order {
		if try(otherStep{kind: otherLife, ans: c}) {
			return true
		}
	}
	return false
}
