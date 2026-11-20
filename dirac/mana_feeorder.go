package dirac

// gorge's generic mana-fee order (rules potentialPrefixScript's phased run with
// scriptSearch.feeOrder): when the plain run finds no script for a plain cast
// beside a census gap that makes two or more mana for a mana fee (Baxter
// Building's {4},{T}: four mana in any combination), the phased run walks the
// covered funding first, then those fee gaps, then the other gaps. The plain
// order taps the gap's free {C} first and spends its node budget on the
// funding sources' orderings below it, so a board of several funders (a land
// whose restricted second ability adds a branch to every tap) reaches its
// budget before the fee gap's activation; the fee order floats the fee first.
//
// gorge reaches it only outside phasedScope (a cast that sacrifices or returns
// a permanent, or a hybrid-fee gap, takes the plain phased order, which the
// kernel does not mirror), so those plays keep phasedFallback's reading.

// feeOrderShape is visit's pass shape under the fee order: pass 0 the covered
// funding (skipped once the fee is funded, the later passes still run), pass 1
// the fee gaps (freeOnly names them), pass 2 the other gaps.
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap, freeOnly, skip bool) {
	if pass == 0 {
		return false, false, funded >= fs.fees
	}
	return true, pass == 1, false
}

// feeOrderGaps is the census gaps of fs that make two or more mana for a mana
// fee (gorge's scriptSearch.feeGaps), and whether the fee order takes play
// fs.a at all: a plain cast with no object part, a filter-only gap set without
// a hybrid fee, and at least one such gap.
func (s *State) feeOrderGaps(k *prep, fs *filtSearch) (slotMask, bool) {
	a := fs.a
	if fs.pay != nil || fs.census || fs.other.any() || a.Kind() != AkCast || a.Mode() != 0 || s.objPartCast(k, a) || s.filtHybIn(k, fs.gaps) {
		return slotMask{}, false
	}
	var fee slotMask
	for b := fs.gaps; b.any(); b = b.rest() {
		if sl := b.first(); s.filtUnits(k, sl) >= 2 && s.filtFee(k, sl) > 0 {
			fee = fee.with(sl)
		}
	}
	return fee, fee.any()
}

// filtFallback runs gorge's phased run after the plain run fs found no script
// and reports whether it found one (fs then holds that run's path). Under the
// fee order it is the whole fallback; otherwise it is the plain run's node
// budget spent on a repeatable fee-only filter's branches (phasedFallback):
// the phased run walks the funding line the plain order walked past.
func (s *State) filtFallback(k *prep, t *State, fs *filtSearch) bool {
	ph := filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
		usefulOK: fs.usefulOK, other: fs.other, feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead}
	if fee, ok := s.feeOrderGaps(k, fs); ok {
		ph.feeOrder, ph.feeGaps = true, fee
	} else if fs.phasedFallback() {
		ph.phased = true
	} else {
		return false
	}
	if ph.visit(k, t, 0, false) {
		*fs = ph
		return true
	}
	if fs.stall == 0 {
		fs.stall = ph.stall
	}
	return false
}
