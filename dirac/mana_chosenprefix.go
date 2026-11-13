package dirac

import "math/bits"

// A chosen-colour widened pair (Sunken Citadel's land-only {C}{C} beside its
// {C}) is no unit of gorge's census, so a play the census alone cannot pay is
// a potential play whose prefix search taps the pair first (CR 601.2g, 602.2b
// allow either order): the oracle plays the prefix before the play's
// targets, floating the restricted mana (State.rb). planChosenTrial pays the
// same play with the pair in the payment, which taps the source too late.
// chosenPairPrefix plays gorge's order: the pair sources, in battlefield
// order, tapped at the play until the census and the floating mana pay it.

// chosenPairPrefix floats the chosen-colour pairs p's activation a needs.
func (s *State) chosenPairPrefix(k *prep, a Action, p uint8) {
	cost, ex, _, ok := s.grantPlayCost(k, p, a, true)
	if !ok {
		return
	}
	d := s.grantPayDesc(k, a)
	d.noPair = true
	if s.affordsUngrantedPlay(k, p, a, &cost, ex, d) {
		return // the census pays it: no prefix search
	}
	d.noPair = false
	if !s.affordsUngrantedPlay(k, p, a, &cost, ex, d) {
		return // not a pair's play (a grant or a script pays it)
	}
	var gaps slotMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if sl == ex || k.gen.flags[c]&gfRestrSrc == 0 || !s.chosenPairSrc(k, c) {
			continue
		}
		if pm := &s.perms[sl]; pm.flags&pfTapped != 0 || s.activationSick(k, sl) || s.genLost(k, sl) {
			continue
		}
		gaps = gaps.with(sl)
	}
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, gaps, order[:], 0)
	d.noPair = true
	for i := 0; i < n && !s.affordsUngrantedPlay(k, p, a, &cost, ex, d); i++ {
		sl := order[i]
		c := s.cardAt(sl)
		rec := k.gen.restrRecOf(c)
		ab := &k.t.Cards[c].Abilities[rec.ab]
		col := bits.TrailingZeros8(s.restrWideMask(&k.gen.rs.views[rec.view], sl) & 0x1f)
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		if !s.floatRestrOnly(k, p, sl, ab, col, max(ab.ManaN, 1)) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
	}
}

// chosenPairSrc: card c's restricted ability is a chosen-colour pair.
func (s *State) chosenPairSrc(k *prep, c CardID) bool {
	rec := k.gen.restrRecOf(c)
	return rec != nil && rec.kind == rkWiden && k.gen.rs.views[rec.view].wideChosen
}
