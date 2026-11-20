package dirac

// Gorge's fee-order fallback run (rules scriptSearch.feeOrder, the phased
// build of potentialPrefixScript for a plain cast): when the plain prefix
// search finds no script, gorge walks the covered funding sources first, then
// the gaps whose mana ability costs mana and makes several units (Baxter
// Building's {4},{T}: four mana in any combination), then the fee-free gaps.
// A fee gap's fee is paid from the floating pool (CR 601.2b, 605.3b), so
// funding it first reaches in a few nodes the plan the plain order (gaps
// first) spends its whole node budget walking past: four lands, then Baxter
// Building turning their four mana into the {G} a Restorative Technique
// needs. The run is a fallback only, as gorge's: the plain run keeps the node
// walk every other search shares a budget with.

// feeGapAt: the filter gap at sl carries a fee and makes more than one unit
// (gorge's feeGaps: a one-unit fee source is a plain filter, priced by the
// fee-only paths).
func (s *State) feeGapAt(k *prep, sl Slot) bool {
	return s.filtFee(k, sl) > 0 && s.filtUnits(k, sl) >= 2
}

// feeOrderScope reports whether gorge's fallback run for play a of p is the
// fee-order one: a plain cast of a spell with no additional cost (a sacrifice
// or return cost keeps gorge's ordinary phased run, which dirac models only
// for repeatable fee-only filters), no hybrid-fee gap, and a fee gap on the
// board.
func (s *State) feeOrderScope(k *prep, a Action, gaps slotMask) bool {
	if a.Kind() != AkCast {
		return false
	}
	if _, _, ok := s.playBaseCost(k, a); !ok {
		return false
	}
	if spellAddCost(k, s.objs[a.Source()].card).Flags != 0 || s.filtHybIn(k, gaps) {
		return false
	}
	for b := gaps; b.any(); b = b.rest() {
		if s.feeGapAt(k, b.first()) {
			return true
		}
	}
	return false
}

// feeOrderRun is the fee-order run's search: the plain run's inputs, walked
// in the fee order, without the plain run's fee-stall prune.
func (fs *filtSearch) feeOrderRun() filtSearch {
	return filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
		usefulOK: fs.usefulOK, other: fs.other, feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead, feeOrder: true}
}

// feeOrderShape is passShape for the fee-order run: pass 0 the covered
// funding (skipped once the funding is spent, the later passes go on: gorge's
// phase loop continues there), pass 1 the fee gaps, pass 2 the fee-free ones;
// the last pass is the life-only sources'.
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap, freeOnly, skip bool) {
	fs.feePass = uint8(pass)
	return pass != 0, false, pass == 0 && funded >= fs.fees
}

// feeOrderSkips: in the fee-order run's gap passes sl is walked only by its
// own pass, the fee gaps' or the free ones'.
func (fs *filtSearch) feeOrderSkips(k *prep, t *State, sl Slot) bool {
	return fs.feeOrder && (fs.gaps.has(sl) && t.feeGapAt(k, sl)) != (fs.feePass == 1)
}
