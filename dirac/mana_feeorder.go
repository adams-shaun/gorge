package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gapSel is the gaps a visit pass walks: all of them, the gaps with a free
// {T} ability only (the phased run's first gap pass), or, in the feeOrder
// run, the multi-unit fee gaps or every other gap.
type gapSel uint8

const (
	selAll gapSel = iota
	selFree
	selFee
	selNoFee
)

// feeOrderGaps is the census gaps of the play's search whose filter makes two
// or more mana for a mana fee (gorge's scriptSearch.feeGaps: Baxter Building's
// {4},{T} for four mana in any combination), when the play is one whose phased
// run takes the feeOrder (potentialPrefixScript: a plain cast, no hybrid-fee
// gap). A one-unit filter (Surveillance Room's {1},{T}) nets no mana from its
// fee, so it is no fee gap. Empty: the phased run is the ordinary one.
func (fs *filtSearch) feeOrderGaps(k *prep, t *State) slotMask {
	if fs.a.Kind() != AkCast || fs.a.Mode() != 0 || fs.pay != nil {
		return slotMask{}
	}
	var out slotMask
	for b := fs.gaps; b.any(); b = b.rest() {
		sl := b.first()
		if kop.HybridFilterManaAb(t.filtAb(k, sl)) {
			return slotMask{} // a hybrid-fee gap is the ordinary phased run's scope
		}
		if t.filtUnits(k, sl) >= 2 && t.filtFee(k, sl) > 0 {
			out = out.with(sl)
		}
	}
	return out
}

// feeOrderShape is visit's pass walk in the feeOrder run (gorge's phaseAt):
// the covered funding, the fee gaps, the other gaps. A spent funding skips
// its pass only: the fee gaps that follow pay their fee from it.
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap bool, sel gapSel, skip bool) {
	switch pass {
	case 0:
		return false, selAll, funded >= fs.fees
	case 1:
		return true, selFee, false
	}
	return true, selNoFee, false
}

// feeOrderSkips: gap sl is not this feeOrder pass's (a fee gap walks in its
// own pass, ahead of the others).
func (fs *filtSearch) feeOrderSkips(sl Slot, sel gapSel) bool {
	return fs.feeOrder && (sel == selFee || sel == selNoFee) && fs.feeGap.has(sl) != (sel == selFee)
}

// feeFloated: the floating pool pays the filter at sl its mana fee, the gate
// gorge's phased run puts on every gap activation (payable: the payer cannot
// legally activate it at the pool so far). The pool's total is the read, a
// pip fee's colour left to the step.
func (fs *filtSearch) feeFloated(k *prep, t *State, sl Slot) bool {
	return poolTotal(&t.pl[SeatIx(fs.p)]) >= t.filtFee(k, sl)
}
