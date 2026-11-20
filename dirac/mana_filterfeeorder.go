package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gorge's prefix search (potentialPrefixScript, potential_script.go) has a
// phased fallback in a "fee order" (scriptSearch.feeOrder): for a plain cast
// whose census gaps include a filter that makes several mana for a mana fee
// (Baxter Building's {4},{T}: four mana, Shadowblood Ridge's {1},{T}: {B}{R}),
// it walks the covered funding first, then those fee-carrying gaps, then the
// fee-free gaps, so the fee is funded before a free any-colour gap floats the
// play's own pip. The plain run spends its node budget on fee branches before
// the fee is funded; the fee order reaches the paying line in a few nodes.

// Which gaps a fee-order pass walks.
const (
	feeSelAny  = iota // not a fee-order run
	feeSelFee         // the multi-mana fee filters
	feeSelFree        // every other gap
)

// feeGapAb reports whether a is a filter gorge's feeGaps marks: it costs mana
// and makes more than one unit (a literal amount; a dynamic one is not read).
func feeGapAb(a *kop.Ability) bool {
	return a.Cost.Mana.MV() > 0 && a.ManaN >= 2 && a.AFlags&kop.AfManaDynamic == 0
}

// feeGapSlot: the gap at sl is a fee-carrying one of gorge's feeGaps (only a
// slot in gaps has a filter ability to read).
func (s *State) feeGapSlot(k *prep, sl Slot, gaps slotMask) bool {
	return gaps.has(sl) && feeGapAb(s.filtAb(k, sl))
}

// feeGapsLive: some gap of the search is a fee-carrying filter.
func (fs *filtSearch) feeGapsLive(k *prep, t *State) bool {
	for b := fs.gaps; b.any(); b = b.rest() {
		if t.feeGapSlot(k, b.first(), fs.gaps) {
			return true
		}
	}
	return false
}

// feeOrderFallback: the plain run found no script after spending its node
// budget, the play is a plain cast of the printed cost, and a gap is a
// fee-carrying filter: gorge's phased run takes the fee order there.
func (fs *filtSearch) feeOrderFallback(k *prep, t *State) bool {
	return fs.pay == nil && !fs.phased && fs.nodes >= potentialScriptNodes &&
		fs.a.Kind() == AkCast && fs.a.Mode() == 0 && fs.feeGapsLive(k, t)
}

// feeOrderShape is visit's pass at index pass of the fee-order run: the
// covered funding (skipped once it is spent, where gorge's phase loop moves
// on), the fee-carrying gaps, then the other gaps; the last pass is the
// life-only source's, as in the other runs.
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap bool, sel int, skip bool) {
	switch pass {
	case 0:
		return false, feeSelAny, funded >= fs.fees
	case 1:
		return true, feeSelFee, false
	}
	return true, feeSelFree, false
}
