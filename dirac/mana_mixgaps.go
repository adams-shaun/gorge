package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Mixed census gaps. gorge's prefix search (rules scriptSearch.visit) activates
// every source its census misses, whatever makes it a gap, depth first in
// priority-option order, and funds a filter's fee from the others only after
// the gaps. filtSearch is that search over filters; on a board that mixes a
// filter, a tap-another-permanent source and a restricted any-colour source
// (Interdimensional Web Watch beside Daily Bugle Building, or beside Supportive
// Parents), and on any filter board with a granted mana ability (Honest Work's
// {C}), the other gaps are its otherGap steps: their answers branched on in
// gorge's order, their asks charged to the node budget.

const availNoWide uint8 = 16 // the gaps walk without a wide source's mana (availAnyR)

const (
	otherNone uint8 = iota
	otherAnyR
	otherTapX
	otherGrant
)

// otherStep is one activation of a gap that is no filter.
type otherStep struct {
	kind        uint8
	ans         uint8 // otherAnyR: the scriptDFS answer; otherTapX, otherGrant: the colour
	n           uint8 // otherGrant: the mana it makes
	gi          int16 // otherGrant: the grantPC index; -1 the source's own ability
	cand, cand2 Slot  // otherTapX: the permanents its cost taps
}

// mixBoard: p's census gaps mix filters, restricted any-colour sources and
// tap-another-permanent sources, and hold nothing else.
func (s *State) mixBoard(k *prep, p uint8) bool {
	if k.gen == nil || !k.gen.anyFilt && !k.tapMana || !k.gen.anyAnyR && !k.tapMana {
		return false
	}
	n := 0
	if s.scriptFilt(k, p).any() {
		n++
	}
	if s.scriptAnyR(k, p).any() {
		n++
	}
	if s.scriptTapX(k, p).any() {
		n++
	}
	return n >= 2 && !s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptCtr(k, p)).any()
}

// otherGaps: the gaps a filter search on p's board activates beside its filters.
func (s *State) otherGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if s.mixBoard(k, p) {
		m = s.scriptAnyR(k, p).or(s.scriptTapX(k, p))
	}
	if len(k.gen.grantPC) != 0 && s.grantsLive(k) {
		for b := s.ctrlM[p&1].and(s.occ); b.any(); b = b.rest() {
			if sl := b.first(); s.grantIdx(k, sl) >= 0 && s.canTapForMana(k, sl) {
				m = m.with(sl)
			}
		}
	}
	return m.andNot(s.scriptFilt(k, p))
}

// otherGap branches on gap sl's answers at a priority node of the search.
func (fs *filtSearch) otherGap(k *prep, t *State, sl Slot, funded int) bool {
	p := fs.p
	try := func(o otherStep) bool {
		return fs.try(k, t, filtStep{sl: sl, gap: true, other: o}, funded, true)
	}
	switch {
	case t.scriptAnyR(k, p).has(sl):
		if !fs.tick() {
			return false
		}
		c := t.cardAt(sl)
		if restrWideAnyMana(&k.t.Cards[c].Abilities[k.gen.anyR(c)]) {
			// the two-unit ask: per-unit answers, W W, W U ... G G, a mirrored
			// pair charging its node and finding the state seen
			ans := uint8(0)
			for a := 0; a < 5; a++ {
				for b := 0; b < 5; b++ {
					if b < a {
						if !fs.tick() {
							return false
						}
						continue
					}
					if try(otherStep{kind: otherAnyR, ans: ans}) {
						return true
					}
					ans++
				}
			}
			return false
		}
		// its {C} first, then the colour ask (scriptDFS's six answers)
		if try(otherStep{kind: otherAnyR}) {
			return true
		}
		if !fs.tick() {
			return false
		}
		for c := uint8(1); c <= 5; c++ {
			if try(otherStep{kind: otherAnyR, ans: c}) {
				return true
			}
		}
	case t.scriptTapX(k, p).has(sl):
		var cs [MaxPerm]Slot
		nc := t.tapXCands(k, p, sl, &cs)
		colours := func(a, b Slot) bool {
			if !fs.tick() { // the colour ask
				return false
			}
			for col := uint8(0); col < 5; col++ {
				if try(otherStep{kind: otherTapX, ans: col, cand: a, cand2: b}) {
					return true
				}
			}
			return false
		}
		if c := t.cardAt(sl); k.t.Cards[c].Abilities[k.cards[c].manaAb].Cost.N == 2 {
			// two permanents: no ask for exactly two, else one "choose 2" whose
			// per-unit answers scriptAnswers enumerates (tapXSearch.visit)
			if nc > 2 && !fs.tick() {
				return false
			}
			per := nc / 2
			if nc == 2 {
				per = 1
			}
			n := 0
			for i := 0; i < per && nc%2 == 0; i++ {
				for j := 0; j < per && n < 64; j++ {
					n++
					a, b := cs[i], cs[1]
					if nc > 2 {
						b = cs[per+j]
					}
					if colours(a, b) {
						return true
					}
				}
			}
			return false
		}
		if nc > 1 && !fs.tick() { // the creature ask
			return false
		}
		for i := 0; i < nc; i++ {
			if colours(cs[i], NoSlot) {
				return true
			}
		}
	case t.canTapForMana(k, sl):
		// several abilities: the "choose a mana ability" ask, one node, the
		// printed ability's answers first (grantSrch.visit)
		var gi [maxGrants]int16
		ng := t.grantIdxs(k, sl, &gi)
		dual := t.ownManaUsable(k, sl)
		alts := ng
		if dual {
			alts++
		}
		if alts > 1 && !fs.tick() {
			return false
		}
		colours := func(g int16, mask, n uint8, ask bool) bool {
			if mask == 0 {
				mask = 1 << kop.C
			}
			if ask && mask&(mask-1) != 0 && !fs.tick() {
				return false
			}
			for m := mask; m != 0; m &= m - 1 {
				if try(otherStep{kind: otherGrant, ans: uint8(bits.TrailingZeros8(m)), n: n, gi: g}) {
					return true
				}
			}
			return false
		}
		if dual {
			pc := t.manaPCGen(k, sl)
			if colours(-1, t.srcMaskAt(pc, sl), pc.manaN, alts == 1) {
				return true
			}
		}
		d := t.grantPayDesc(fs.a)
		for j := 0; j < ng; j++ {
			if r := k.gen.grantOf[gi[j]]; k.gen.grantRestr[gi[j]] != 0 && !t.restrSrcAdmits(k, p, sl, &k.t.Cards[r.card].Abilities[r.ab], &d) {
				continue
			}
			pc := t.grantView(k.gen, gi[j], sl)
			mask := t.srcMaskAt(pc, sl)
			if colours(gi[j], mask, pc.manaN, alts == 1 || mask&uint8(kop.ColAny) == uint8(kop.ColAny)) {
				return true
			}
		}
	}
	return false
}

// otherDo plays step st of a gap that is no filter.
func (s *State) otherDo(k *prep, p uint8, st filtStep) {
	o := &st.other
	switch o.kind {
	case otherAnyR:
		var spill [kop.NumManaKinds]uint8
		s.tapGapChoice(k, p, st.sl, slotMask{}, slotMask{}.with(st.sl), int(o.ans), &spill)
		if spill != ([kop.NumManaKinds]uint8{}) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
	case otherTapX:
		s.tapXDo(k, p, tapXStep{sl: st.sl, cand: o.cand, cand2: o.cand2, col: o.ans})
	case otherGrant:
		s.perms[st.sl].flags |= pfTapped
		s.touchSl(st.sl)
		if o.gi >= 0 && k.gen.grantRestr[o.gi] != 0 {
			if !s.pl[p&1].addRestr(k, &s.poolRx[p&1], k.gen.grantRestr[o.gi], int(o.ans), o.n) {
				s.capFull(StallRestrSrc)
			}
			return
		}
		s.pl[p&1].pool[o.ans] += o.n
		s.artTag(k, p, st.sl, int(o.ans))
	}
}

// filtFeeRestrN is how much of p's floating restricted mana filter sl's fee may
// spend: the fee is an activated ability's payment (Hydraulic Helper's mana
// "can't be spent to cast a nonartifact spell", so it can pay it).
func (s *State) filtFeeRestrN(k *prep, p uint8, sl Slot) int {
	if s.pl[p&1].poolR == 0 {
		return 0
	}
	d := payDesc{class: pdAct, card: s.cardAt(sl), src: sl}
	if !s.restrPoolAct(k, p, &d) {
		return 0
	}
	var t avail
	s.restrAvailAdd(p, &t)
	return t.total
}

// filtFeeRestr spends up to fee of that mana and returns how much it spent:
// gorge carves the batches the payment may spend before ordinary mana.
func (s *State) filtFeeRestr(k *prep, p uint8, sl Slot, fee int) int {
	left := min(fee, s.filtFeeRestrN(k, p, sl))
	spent := left
	pl := &s.pl[p&1]
	if n := min(left, int(pl.poolR&poolRCnt)); n > 0 {
		left -= n
		if c := int(pl.poolR&poolRCnt) - n; c > 0 {
			pl.poolR = pl.poolR&^poolRCnt | uint8(c)
		} else {
			pl.poolR = 0
		}
	}
	for c := 0; left > 0 && c < 5; c++ {
		n := min(left, int(s.poolRx[p&1]>>(3*c))&7)
		s.poolRx[p&1] -= uint16(n) << (3 * c)
		left -= n
	}
	return spent
}
