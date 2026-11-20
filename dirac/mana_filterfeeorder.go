package dirac

// gorge's phased fallback run (engine/rules/potential_script.go
// potentialPrefixScript's second build) takes a third order for a plain cast
// beside a multi-unit fee-carrying filter gap (scriptSearch.feeOrder; Baxter
// Building's "{4}, {T}: Add four mana in any combination of colors"): the
// covered funding sources first, then the fee gaps (their fee is paid from
// the floating pool, CR 601.2b), then the fee-free gaps. The plain order taps
// the gap's own free {C} first and spends its node budget on the funding
// branches (colour asks of Training Compound and Asgardian Citadel) before the
// allocation that pays, so a cast the plain run budget-limits on is found by
// this run in a handful of nodes and offered (feeGaps, phasedScope).

// gapFilter is which gaps a gap pass of filtSearch.visit walks.
type gapFilter uint8

const (
	gapAll      gapFilter = iota // every gap
	gapFreeOnly                  // the phased run's first gap pass: the gaps whose ability costs no mana
	gapFeeOnly                   // the fee-order run's fee-carrying gaps
	gapNoFee                     // the fee-order run's other gaps
)

// feeGapAt reports whether the filter at sl is a fee gap of gorge's
// scriptSearch.feeGaps: its mana ability costs mana and makes more than one
// unit (a one-unit fee source is a filter whose net production is zero).
func (s *State) feeGapAt(k *prep, sl Slot) bool {
	return s.filtUnits(k, sl) >= 2 && s.filtFee(k, sl) > 0
}

// feeGapsIn reports whether some filter of the search's gaps is a fee gap.
func (s *State) feeGapsIn(k *prep, f slotMask) bool {
	for ; f.any(); f = f.rest() {
		if s.feeGapAt(k, f.first()) {
			return true
		}
	}
	return false
}

// feeOrderFallback reports whether gorge's phased run takes the fee order
// for this search: a plain cast of the printed cost with a fee gap and no
// hybrid-fee gap (phasedScope would take that board's own phased order),
// beside filters alone and not the repeatable fee-only class, which keeps
// phasedFallback's run.
func (fs *filtSearch) feeOrderFallback(k *prep, s *State) bool {
	a := fs.a
	return a.Kind() == AkCast && a.Mode()&3 == 0 && a.Mode()&(castMayPlay|castFace|castWarp) == 0 &&
		!fs.feeOnly && !fs.other.any() && fs.pay == nil && !fs.phased &&
		s.feeGapsIn(k, fs.gaps) && !s.filtHybIn(k, fs.gaps)
}

// fallbackRun reports whether the plain run's failure (found false) is
// followed by gorge's phased run, and whether that run is the fee order.
func (fs *filtSearch) fallbackRun(k *prep, s *State, found bool) (feeOrder, run bool) {
	if found {
		return false, false
	}
	if fs.phasedFallback() {
		return false, true
	}
	return true, fs.feeOrderFallback(k, s)
}

// phasedRun is the fallback run's search: the plain run's setup with the
// phased order (or, feeOrder, the fee order).
func (fs *filtSearch) phasedRun(feeOrder bool) filtSearch {
	return filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
		usefulOK: fs.usefulOK, other: fs.other, feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead, phased: true, feeOrder: feeOrder}
}

// feeOrderShape is passShape's for the fee-order run: the funding pass, the
// fee gaps, the other gaps (gorge's funded >= funding continue skips the
// funding pass only).
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap bool, gf gapFilter, skip bool) {
	switch pass {
	case 0:
		return false, gapAll, funded >= fs.fees
	case 1:
		return true, gapFeeOnly, false
	}
	return true, gapNoFee, false
}

// feeOrderSkips reports whether gap pass gf leaves the filter at sl to the
// other gap pass of the fee-order run.
func (s *State) feeOrderSkips(k *prep, sl Slot, gf gapFilter) bool {
	return gf >= gapFeeOnly && s.feeGapAt(k, sl) != (gf == gapFeeOnly)
}
