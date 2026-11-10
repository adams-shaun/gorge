package kgen

// rememberedHoldsOne reports whether the resolution's remembered set, as the
// program so far wrote it, holds at most one object: exactly one writer, and
// that writer is a hand-to-battlefield choice of the constant one card
// (ChooseMove with Remember, Farragut's "put a creature card from your hand
// onto the battlefield"). A triggered ability whose chain wrote such a set
// registers its delayed program over that one object; gorge drops the
// trigger's own capture once the chain has remembered something. Any other
// writer (a dig, an exile of the top X cards) can leave several objects,
// which the delayed record cannot carry.
func (lc *lowerCtx) rememberedHoldsOne() bool {
	if lc.ab == nil {
		return false
	}
	writers := 0
	ok := false
	for _, in := range lc.ab.Prog {
		if !kremWrites(in) {
			continue
		}
		writers++
		ok = in.Op == "ChooseMove" && in.A.isConst() && in.A.N == 1
	}
	return writers == 1 && ok
}

// rememberedFromDig reports whether a dig of the program so far wrote the
// remembered set: it holds as many cards as the dig takes, which the delayed
// record cannot carry (Dragonhawk's exiled top X cards).
func (lc *lowerCtx) rememberedFromDig() bool {
	if lc.ab == nil {
		return false
	}
	for _, in := range lc.ab.Prog {
		if in.Op == "Dig" && kremWrites(in) {
			return true
		}
	}
	return false
}
