package dirac

// The phased run's generic mana-fee order (gorge potentialPrefixScript's
// scriptSearch.feeOrder). After the plain prefix search finds no script, gorge
// runs its phased search, and for a plain cast beside a census gap whose mana
// ability costs mana and makes more than one unit (Baxter Building's "{4}, {T}:
// add four mana in any combination") the phased run walks the covered funding
// first, then those fee-carrying gaps, then the fee-free gaps: the fee is a mana
// payment settled from the floating pool (CR 605.3b, 601.2h), so floating it
// before any other gap has floated the play's own mana leaves each its colour.
// The plain order spends its whole node budget on the free gaps' and the
// restricted sources' answers (a Villainous Hideout beside the funding Mountains
// has seven) before the funding line is reached, so only this order prices
// the play.

// feeOrderGaps is the gaps whose mana ability costs mana and makes more than
// one unit (gorge scriptSearch.feeGaps): a one-unit fee source is a filter whose
// net production is zero, which the fee-only paths price.
func (s *State) feeOrderGaps(k *prep, gaps slotMask) slotMask {
	var out slotMask
	for b := gaps; b.any(); b = b.rest() {
		sl := b.first()
		if s.filtFee(k, sl) > 0 && s.filtUnits(k, sl) >= 2 {
			out = out.with(sl)
		}
	}
	return out
}

// feeOrderScope reports whether the fee order prices play a: a plain cast of
// the printed cost on a board with no other kind of gap and no hybrid-fee
// filter (gorge's hybrid-fee scope keeps its own phased walk), whose plain run
// spent its whole node budget. gorge also runs the phased order after a plain
// run that completed without a script; the gates it adds find nothing a
// completed plain search missed, so only the budget-limited one is walked.
func (fs *filtSearch) feeOrderScope(s *State, k *prep) slotMask {
	if fs.pay != nil || fs.other.any() || fs.a.Kind() != AkCast || fs.a.Mode() != 0 || fs.nodes < potentialScriptNodes || s.filtHybIn(k, fs.gaps) {
		return slotMask{}
	}
	return s.feeOrderGaps(k, fs.gaps)
}

// feeOrderShape is passShape's feeOrder arm: pass 0 the covered funding (while
// funding lasts), pass 1 the fee-carrying gaps, pass 2 the fee-free ones. The
// gap passes are not cut once the funding is spent: the fees that follow are
// paid from it (gorge's feeOrder `continue`, not the phased break).
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap, freeOnly, skip bool) {
	switch pass {
	case 0:
		return false, false, funded >= fs.fees
	case 1:
		return true, false, false
	}
	return true, true, false
}

// feeOrderSkip is skipSlot's feeOrder arm for a gap pass: the fee-carrying pass
// (freeOnly false) walks fs.feeGaps, the other pass the rest.
func (fs *filtSearch) feeOrderSkip(sl Slot, freeOnly bool) bool {
	return fs.feeGaps.has(sl) == freeOnly
}
