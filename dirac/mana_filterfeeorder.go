package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// feeOrder is gorge's third build of potentialPrefixScript, the phased run's
// generic mana-fee order (scriptSearch.feeOrder): a plain cast whose census
// gaps include a multi-unit fee filter is searched, once the plain run found
// no script, with the covered funding first, then the fee-carrying gaps, then
// the fee-free gaps. The plain run walks the fee-free gaps first, so a row of
// restricted-colour answers (Avengers Tower's any colour, spendable by no part
// of the cast) spends its 160 nodes before Baxter Building's {4} filter is
// funded; this order funds the filter first (CR 601.2b, 605.3b).

// phasedRun is fs's fallback twin: the same play and census, a fresh budget,
// phased.
func (fs *filtSearch) phasedRun() filtSearch {
	return filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
		usefulOK: fs.usefulOK, other: fs.other, feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead, phased: true}
}

// feeGapMask is the gaps of m whose ability costs mana and makes more than one
// unit (gorge's feeGaps: a one-unit fee source nets zero, so the fee-only
// paths price it and funding it first only changes the node walk). A dynamic
// amount (Starfleet Academy's one mana per creature type) is no unit count
// gorge's projection prices (availableAmount reads 0 for a non-literal
// Amount), so it is never a fee gap.
func (s *State) feeGapMask(k *prep, m slotMask) (out slotMask) {
	for b := m; b.any(); b = b.rest() {
		sl := b.first()
		if s.filtAb(k, sl).AFlags&kop.AfManaDynamic != 0 {
			continue
		}
		if s.filtUnits(k, sl) >= 2 && s.filtFee(k, sl) > 0 {
			out = out.with(sl)
		}
	}
	return out
}

// feeOrderFallback: the plain run found no script and gorge's feeOrder run
// follows: a plain cast of the printed cost (a payment search has no
// fallback, and gorge limits it to o.Mode == "" and the printed cost) with a
// fee-carrying filter gap and no gap beside the filters, and no sacrifice or return in the cast's own cost. A hybrid-fee filter
// is the phased scope's (mana_filterhybrid.go), which this order leaves alone.
func (fs *filtSearch) feeOrderFallback(s *State, k *prep) bool {
	if fs.pay != nil || fs.a.Kind() != AkCast || fs.a.Mode() != 0 || fs.other.any() {
		return false
	}
	// A cast whose own cost sacrifices or returns a permanent is the phased
	// scope's already (gorge phasedScope), which runs its ordinary phased order.
	if f := spellAddCost(k, s.objs[fs.a.Source()].card).Flags; f&(kop.CostSac|kop.CostReturn) != 0 {
		return false
	}
	return s.feeGapMask(k, fs.gaps).any() && !s.filtHybIn(k, fs.gaps)
}

// feeOrderShape is passShape under feeOrder: pass 0 funds, pass 1 walks the
// fee-carrying gaps, pass 2 the fee-free ones, pass 3 the life source. The
// funding pass ends once the funding is spent, and the gaps that follow pay
// from what floated (gorge's phase loop continues rather than breaks).
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap, freeOnly, skip bool) {
	if pass == 0 {
		return false, false, funded >= fs.fees
	}
	return true, false, false
}

// feeOrderSkip: under feeOrder, the gap pass that is not sl's: the fee-carrying
// gaps are pass 1, the fee-free ones pass 2.
func (fs *filtSearch) feeOrderSkip(pass int, sl Slot) bool {
	return fs.feeOrder && pass != 0 && fs.feeGaps.has(sl) != (pass == 1)
}
