package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A sacrifice-any mana source (Ashnod's Altar, no {T}) beside a plain granted
// {T} mana ability (Elven Chorus's "{T}: Add one mana of any color" on every
// creature). gorge's planner census holds printed mana abilities only
// (rules/pay PaymentAbility), so a grant is a census gap its ONE prefix search
// activates in battlefield order beside the Altar (scriptSearch.visit: the
// gaps first, the play re-priced after each; it takes the Altar again for as
// long as it has a creature to sacrifice before the later grant holders). The
// grant search (grantSrch) does not activate a sacrifice-another source
// (tapXOffGrant), so the tap-another search takes the grant holders as gaps
// of its own, a step each (tapXGrant): the holder taps and its colour floats.
// Only plain grants are modelled: one unrestricted, costless {T} ability on a
// holder that has no mana ability of its own; any other grant keeps the
// tap-another search pricing the printed sources alone.

// tapXGrantBoard: play a is an activation of p on a board with a live
// sacrifice-any source and a plain granted mana ability.
func (s *State) tapXGrantBoard(k *prep, p uint8, a Action) bool {
	if a.Kind() != AkActivate || !k.tapMana || !s.sacXLive(k, p) {
		return false
	}
	_, ok := s.tapXGrantGaps(k, p, slotMask{})
	return ok
}

// tapXGrantGaps: p's untapped holders of a plain granted {T} mana ability, the
// gaps the tap-another search adds beside its own. ok is false when a live
// holder's grant is not plain (the search then adds none).
func (s *State) tapXGrantGaps(k *prep, p uint8, have slotMask) (m slotMask, ok bool) {
	if k.gen == nil || !k.gen.anyContGrant || s.restrictedGrantLive(k, p) {
		return m, false
	}
	for b := s.ctrlM[SeatIx(p)].and(s.occ).andNot(have); b.any(); b = b.rest() {
		sl := b.first()
		if s.grantIdx(k, sl) < 0 || !s.grantGapUsable(k, sl) {
			continue
		}
		if _, _, plain := s.tapXGrantOf(k, sl); !plain {
			return slotMask{}, false
		}
		m = m.with(sl)
	}
	return m, m.any()
}

// tapXGrantOf is the granted index of sl's one plain {T} mana ability, and
// whether the source also has a plain printed one (a land an effect animated:
// gorge's ask lists the printed ability first, then the grant).
func (s *State) tapXGrantOf(k *prep, sl Slot) (gi int16, dual, ok bool) {
	var gis [maxGrants]int16
	if s.grantIdxs(k, sl, &gis) != 1 {
		return 0, false, false
	}
	gi = gis[0]
	pc := s.grantView(k.gen, gi, sl)
	if !pc.manaTap || pc.grantSac || pc.manaSac || pc.manaSacOnly || grantCostLife(pc) > 0 || k.gen.grantRestr[gi] != 0 || pc.manaMulti || pc.manaChosen {
		return 0, false, false
	}
	if !s.ownManaUsable(k, sl) {
		return gi, false, k.cards[s.cardAt(sl)].manaAb < 0
	}
	ppc, base := s.manaPCGen(k, sl), &k.cards[s.cardAt(sl)]
	mask := s.srcMaskAt(ppc, sl)
	if base.manaAb < 0 || !ppc.manaTap || ppc.manaSac || ppc.manaSacOnly || ppc.manaMulti || ppc.manaChosen || mask&(mask-1) != 0 ||
		mask != srcMask(base) || s.ownRestrAb(k, sl) != nil || grantPainShape(k, ppc) {
		return 0, false, false
	}
	return gi, true, true
}

// grantAsk tries the grant holder sl's step at the node t: with a printed
// ability beside the grant the "choose a mana ability" ask is a node and the
// printed colour comes first; then the grant's colours in gorge's answer
// order, one node for a choice of colours.
func (rs *tapXSearch) grantAsk(k *prep, t *State, sl Slot) bool {
	if !t.grantGapUsable(k, sl) {
		return false
	}
	gi, dual, plain := t.tapXGrantOf(k, sl)
	if !plain {
		return false
	}
	if dual {
		if !rs.tick() { // the ability ask
			return false
		}
		ppc := t.manaPCGen(k, sl)
		col := uint8(bits.TrailingZeros8(t.srcMaskAt(ppc, sl)))
		if rs.try(k, t, tapXStep{kind: tapXGrant, sl: sl, cand: NoSlot, cand2: NoSlot, col: col, gi: -1}) {
			return true
		}
	}
	mask := t.srcMaskAt(t.grantView(k.gen, gi, sl), sl)
	if mask == 0 {
		mask = 1 << kop.C
	}
	if mask&(mask-1) != 0 && !rs.tick() { // the colour ask
		return false
	}
	var order [kop.NumManaKinds]uint8
	no := t.stepColours(k, sl, gi, mask, &order)
	if t.stepNamed(k, sl, gi, mask) {
		needFirst(order[:no], rs.need)
	}
	for _, c := range order[:no] {
		if rs.try(k, t, tapXStep{kind: tapXGrant, sl: sl, cand: NoSlot, cand2: NoSlot, col: c, gi: gi}) {
			return true
		}
	}
	return false
}

// tapXGrantDo plays a grant step: the holder taps, the colour floats.
func (s *State) tapXGrantDo(k *prep, p uint8, st tapXStep) {
	s.perms[st.sl].flags |= pfTapped
	s.touchSl(st.sl)
	s.manaTapped(k, st.sl)
	pc := s.manaPCGen(k, st.sl) // the printed ability's (gi -1)
	if st.gi >= 0 {
		pc = s.grantView(k.gen, st.gi, st.sl)
	}
	s.poolAdd(p, int(st.col), int(pc.manaN))
	s.tapExtraStall(k, st.sl)
	s.artTag(k, p, st.sl, int(st.col))
}
