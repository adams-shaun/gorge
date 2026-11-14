package dirac

import "math/bits"

// A chosen-colour pair source (Sunken Citadel: {T}: one mana of the chosen
// colour, and a land-only {C}{C}) beside a filter. The pair is no unit of
// gorge's census, so the source is a census gap, and gorge's prefix search
// (rules scriptSearch.visit) activates every gap in battlefield order before
// it funds a filter's fee from the others: Captivating Cave ({1}, {T}: one
// mana of any colour) beside the Citadel taps the Citadel first, its
// "choose a mana ability" ask answering the plain {T} ability, and that mana
// pays the Cave's fee (CR 601.2g, 602.2b, 605.3a). The filter search takes
// the Citadel as an otherGap step.

// chosenPairGaps: p's activatable chosen-colour pair sources when a filter is
// a census gap beside them. Without a filter the pair is chosenPairPrefix's.
func (s *State) chosenPairGaps(k *prep, p uint8) slotMask {
	if k.gen == nil || k.gen.rs == nil || !k.gen.rs.chosenPair || !s.scriptFilt(k, p).any() {
		return slotMask{}
	}
	var m slotMask
	for b := s.gapMask(k, p, gfRestrSrc); b.any(); b = b.rest() {
		if sl := b.first(); s.chosenPairSrc(k, s.cardAt(sl)) {
			m = m.with(sl)
		}
	}
	return m
}

// chosenPairGap branches chosen-pair gap's answers: the "choose a mana
// ability" ask is one node, its plain {T} ability answered before the pair.
// A tapped source is no candidate (the search visits it once it is tapped).
func (fs *filtSearch) chosenPairGap(t *State, sl Slot, try func(otherStep) bool) bool {
	if t.perms[sl].flags&pfTapped != 0 || !fs.tick() {
		return false
	}
	return try(otherStep{kind: otherChosen, plain: true}) || try(otherStep{kind: otherChosen})
}

// chosenGapDo plays a chosen-pair gap's answer: the plain ability floats one
// mana of the chosen colour, the other the land-only pair (tapChosenPair).
func (s *State) chosenGapDo(k *prep, p uint8, sl Slot, plain bool) {
	if !plain {
		s.tapChosenPair(k, p, sl)
		return
	}
	pc := s.manaPCGen(k, sl)
	col := bits.TrailingZeros8(s.srcMaskAt(pc, sl))
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	s.poolAdd(p, col, int(max(pc.manaN, 1)))
	s.addExtra(k, p, sl, pc)
	s.artTag(k, p, sl, col)
}
