package dirac

// The fee order of gorge's phased prefix run (scriptSearch.feeOrder,
// engine/rules/potential_script.go). A plain cast beside a filter gap whose
// activation costs mana and makes more than one unit (Overflowing Basin's
// {1}, {T}: Add {G}{U}) is priced by the plain run first; when that finds no
// script gorge runs the phased build in a fee order of its own: the covered
// funding, then the fee-carrying gaps (their fee comes out of the floating
// pool, CR 601.2b), then the fee-free ones. The plain order walks every
// Island before the Forest that funds the Basin, so a board whose taps cost
// life (Scald) spends the whole node budget on lines that cannot pay; the fee
// order reaches the funding line first.

// gapSel picks which gaps a pass of visit's activation loop walks.
type gapSel uint8

const (
	selAll   gapSel = iota // every gap
	selFree                // the gaps that carry no fee (the phased run's first gap pass)
	selFee                 // the fee-carrying multi-unit gaps (feeOrder's second pass)
	selNoFee               // the other gaps (feeOrder's last pass)
)

// feeGapAt reports whether the filter gap at sl carries a mana fee and makes
// more than one unit (gorge's feeGaps).
func (s *State) feeGapAt(k *prep, sl Slot) bool {
	return s.filtFee(k, sl) > 0 && s.filtUnits(k, sl) >= 2
}

// gapSelSkips reports whether sel leaves the gap at sl out of the pass.
func (fs *filtSearch) gapSelSkips(k *prep, t *State, sl Slot, sel gapSel) bool {
	switch sel {
	case selFree:
		return !t.canTapForMana(k, sl)
	case selFee:
		return !fs.gaps.has(sl) || !t.feeGapAt(k, sl)
	case selNoFee:
		return fs.gaps.has(sl) && t.feeGapAt(k, sl)
	}
	return false
}

// feeOrdShape is passShape for the fee order: pass 0 the funding (skipped
// once it is spent, as gorge's activations loop continues past it), pass 1
// the fee-carrying gaps, pass 2 the fee-free ones; the last pass is the
// life-only source.
func (fs *filtSearch) feeOrdShape(pass, funded int) (gap bool, sel gapSel, skip bool) {
	switch pass {
	case 0:
		return false, selAll, funded >= fs.fees
	case 1:
		return true, selFee, false
	}
	return true, selNoFee, false
}

// feeOrderFallback reports whether gorge's phased run takes the fee order for
// the play: a plain cast with no additional cost (an own sacrifice or return
// keeps gorge's ordinary phased scope), a filter gap with a fee and several
// units, no hybrid fee among the gaps (the other half of that scope), and no
// non-filter gap but a Taps-deferred one (Islands under Scald: a plain {T} that
// carries no fee, so it joins the fee-free pass; the other kinds' fee class is
// not read here).
func (fs *filtSearch) feeOrderFallback(k *prep, s *State) bool {
	a := fs.a
	if a.Kind() != AkCast || a.Mode()&3 != 0 || a.Mode()&(castMayPlay|castFace|castWarp) != 0 ||
		fs.pay != nil || fs.other.andNot(s.scriptTapsDefer(k, fs.p)).any() || !fs.gaps.any() {
		return false
	}
	if sp := k.t.Cards[s.objs[Obj(a.Source())].card].SpellCost; sp.Flags != 0 || sp.N != 0 || sp.Ctr != 0 {
		return false
	}
	fee := false
	for b := fs.gaps; b.any(); b = b.rest() {
		sl := b.first()
		if _, _, hyb := s.filtHyb(k, sl); hyb {
			return false
		}
		fee = fee || s.feeGapAt(k, sl)
	}
	return fee
}
