package dirac

import "math/bits"

// A filter gap whose source has a restricted any-colour ability too (White
// Lotus Hideout: {T}: Add {C}; {T}: Add one mana of any colour, spent only on
// a Lesson or Shrine spell; {1}, {T}: Add one mana of any colour). gorge's
// prefix search (rules scriptSearch.visit) activates the gap object and
// branches the "choose a mana ability" ask, whose options are the abilities
// payable now in printed order: the {C}, the restricted one (its colour ask
// next) and, once the pool pays its fee, the filter (its colour ask next).
// One node for the ask and one for each colour ask, as the funding branch
// charges a restricted source's (filtSearch.visit). The restricted answer is
// that branch's step (filtRestrDo); filtDeadEnd reads its restriction.

// mixSrcMask is the restricted ability's colours when filter gap sl has one
// (kgen lowers that shape beside a plain {T} one only, refuseMultiMana).
func (t *State) mixSrcMask(k *prep, sl Slot) uint8 {
	if !k.restrSrc {
		return 0
	}
	return t.filtRestrMask(k, sl)
}

// mixSrcGap branches on gap sl's answers at a priority node of the search;
// rm is its restricted ability's colours, filt that the pool pays its
// filter's fee (the ask then lists the filter).
func (fs *filtSearch) mixSrcGap(k *prep, t *State, sl Slot, funded int, rm uint8, filt bool) bool {
	if !fs.tick() { // the ability ask
		return false
	}
	if fs.try(k, t, filtStep{sl: sl, gap: true}, funded, true) {
		return true
	}
	if bits.OnesCount8(rm) > 1 && !fs.tick() { // the restricted ability's colour ask
		return false
	}
	for m := rm; m != 0; m &= m - 1 {
		if fs.try(k, t, filtStep{sl: sl, ans: uint8(bits.TrailingZeros8(m)), restr: true}, funded, true) {
			return true
		}
	}
	return filt && fs.filtColours(k, t, sl, funded)
}
