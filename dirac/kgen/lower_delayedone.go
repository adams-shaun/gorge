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

// dualDelayedReturn reports a ChangeZone Origin$ Graveyard,Exile whose Defined$
// is the delayed registration's remembered object and whose destination is the
// battlefield: gorge moves the object when it lies in either zone (a Return
// whose origin mask holds both), and does nothing once it has gone elsewhere.
// A ChangeType$ or Hidden$ pick is another shape (graveExilePick).
func dualDelayedReturn(ps *params) bool {
	return ps.str("Defined") == "DelayTriggerRememberedLKI" && ps.str("Destination") == "Battlefield" &&
		ps.str("ChangeType") == "" && ps.str("Hidden") == ""
}
