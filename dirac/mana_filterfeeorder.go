package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Gorge's fee-order run (rules/potential_script.go potentialPrefixScript's
// second build with scriptSearch.feeOrder): a plain cast whose census gaps
// include a fee-carrying filter making two or more mana (Baxter Building's
// {4},{T}: four mana) is searched, once the plain run finds nothing, in the
// order funding sources, fee-carrying gaps, the other gaps. The fee is paid
// from the floating pool (CR 601.2b), so the funding floats come before the
// filter that spends them; the plain order's gaps-first walk spends its node
// budget on the permutations of a long funding row before it reaches that
// line (three Forests, a Plains and Castle Doom beside Baxter Building).

// feeSel is the gap pass's class of gaps in the fee-order run: any (every
// other run), the fee-carrying ones, or the rest.
const (
	feeSelAny uint8 = iota
	feeSelFee
	feeSelPlain
)

// feeGapMask is the filters of gaps gorge's feeGaps names: one whose ability
// makes two or more mana and carries a mana fee. A dynamic amount counts as
// none (gorge's availableAmount reads a literal Amount$ only).
func (s *State) feeGapMask(k *prep, gaps slotMask) (m slotMask) {
	for ; gaps.any(); gaps = gaps.rest() {
		sl := gaps.first()
		if s.filtAb(k, sl).AFlags&kop.AfManaDynamic != 0 {
			continue
		}
		if s.filtUnits(k, sl) >= 2 && s.filtFee(k, sl) > 0 {
			m = m.with(sl)
		}
	}
	return m
}

// feeOrderApplies: gorge builds the fee-order run for a plain cast of the
// printed cost with a fee-carrying gap, outside its phased scope (a cost
// that sacrifices a permanent, or a hybrid-fee gap: scriptSearch.phasedScope,
// whose run the plain-limited fallback keeps).
func (s *State) feeOrderApplies(k *prep, a Action, fs *filtSearch) bool {
	if a.Kind() != AkCast || a.Mode() != 0 || !fs.feeGaps.any() || s.filtHybIn(k, fs.gaps) {
		return false
	}
	o := Obj(a.Source())
	return o != NoObj && int(o) < len(s.objs) && spellAddCost(k, s.objs[o].card).Flags&(kop.CostSac|kop.CostSacSelf) == 0
}

// feeOrderPass is passShape's fee-order arm: pass 0 the funding sources
// (skipped, not ended, once the funding is spent), pass 1 the fee-carrying
// gaps, pass 2 the other gaps.
func (fs *filtSearch) feeOrderPass(pass, funded int) (gap, skip bool, sel uint8) {
	switch pass {
	case 0:
		return false, funded >= fs.fees, feeSelAny
	case 1:
		return true, false, feeSelFee
	}
	return true, false, feeSelPlain
}

// feeOrderRun runs the fee-order search from t, the root fork filtScript's
// plain run walked: a fresh search over the same play and gaps.
func (fs *filtSearch) feeOrderRun(k *prep, t *State) (filtSearch, bool) {
	ph := filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
		usefulOK: fs.usefulOK, other: fs.other, feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead,
		phased: true, feeOrder: true, feeGaps: fs.feeGaps}
	found := ph.visit(k, t, 0, false)
	return ph, found
}
