package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The fee-order fallback run (gorge's scriptSearch.feeOrder, built by
// potentialPrefixScript's second build): a plain cast of the printed cost
// beside a multi-unit fee gap (Baxter Building's {4},{T}: four mana in any
// combination) whose plain run spent its whole node budget without a script.
// gorge's phased order then floats the covered funding first, so the gap's
// fee is paid from the pool, then walks the fee-carrying gaps, then the
// fee-free ones. The plain order taps the filter's free {C} ability first
// and spends all 160 nodes funding a line that can no longer make the cost's
// colour; the fee order reaches the filter's colour answer after the funding
// floats, in a handful of nodes.

// feeGapMask is the gaps of fs.gaps gorge's build marks as fee gaps
// (scriptSearch.feeGaps): a mana ability that makes at least two units and
// costs mana.
func (s *State) feeGapMask(k *prep, gaps slotMask) slotMask {
	var m slotMask
	for b := gaps; b.any(); b = b.rest() {
		if sl := b.first(); s.filtAb(k, sl).AFlags&kop.AfManaDynamic == 0 && s.filtUnits(k, sl) >= 2 && s.filtFee(k, sl) > 0 {
			m = m.with(sl)
		}
	}
	return m
}

// feeOrderScope reports whether gorge's second run for play a is the fee
// order: a plain cast of the printed cost with no additional cost (the
// regular phased scope is a sacrifice or return cost, or a hybrid-fee gap),
// beside a fee gap, on a board of filters alone.
func (fs *filtSearch) feeOrderScope(k *prep, s *State, a Action) bool {
	if a.Kind() != AkCast || a.Mode() != 0 || fs.pay != nil || fs.other.any() || fs.nodes < potentialScriptNodes ||
		!fs.gaps.any() || s.filtHybIn(k, fs.gaps) || s.filtRepeatIn(k, fs.gaps) {
		return false
	}
	if spellAddCost(k, s.objs[Obj(a.Source())].card).Flags != 0 {
		return false
	}
	return s.feeGapMask(k, fs.gaps).any()
}

// fallbackRun is filtScript's second run when the plain one found no script:
// gorge's fee-order run, else the phased run of a board with a repeatable
// fee-only filter. It reports whether the run found a script, which then
// replaces fs.
func (fs *filtSearch) fallbackRun(k *prep, s, t *State, a Action) bool {
	var ph filtSearch
	switch {
	case fs.feeOrderScope(k, s, a):
		ph = filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
			usefulOK: fs.usefulOK, other: fs.other, fees: fs.fees, dead: fs.dead, phased: true, feeOrder: true,
			feeGaps: s.feeGapMask(k, fs.gaps)}
	case fs.phasedFallback():
		// gorge's plain run spent its node budget on a repeatable fee-only
		// filter's branches: its phased run walks the funding line the plain
		// order walked past (potentialPrefixScript)
		ph = filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
			usefulOK: fs.usefulOK, other: fs.other, feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead, phased: true}
	default:
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
