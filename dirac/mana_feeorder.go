package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gorge's potentialPrefixScript runs a second, phased search where the plain
// one found no script, and for a plain cast whose census gaps include a
// fee-carrying multi-unit mana ability (Baxter Building's "{4}, {T}: Add four
// mana in any combination of colors") that run takes the generic mana-fee
// order (scriptSearch.feeOrder): the covered funding first, then the
// fee-carrying gaps, then the fee-free ones. The fee is a mana payment settled
// from the floating pool (CR 601.2b, 605.3a), so floating it before the free
// gap's own {C} answer taps the filter's land finds the line the plain order
// spends its 160 nodes walking past (3 Plains, Avengers Hangar and Stark
// Industries beside Baxter Building pay Reptil's {G}). filtScript mirrors it
// with filtSearch.feeOrder; the plain order stays the answer wherever it finds
// a script.

// feeOrderPasses is visit's pass count in the fee order: the covered funding,
// the fee-carrying gaps, the fee-free gaps, then the life-only last resort.
const feeOrderPasses = 4

// castConsumesOwn reports whether cast face cd's own cost sacrifices or
// returns a permanent (CR 601.2b: the activation comes before the sacrifice).
func castConsumesOwn(cd *kop.CardDef) bool {
	const sacOrReturn = kop.CostSac | kop.CostSacSelf | kop.CostReturn | kop.CostReturnSelf
	if cd.SpellCost.Flags&sacOrReturn != 0 || cd.AltAddCost.Flags&sacOrReturn != 0 {
		return true
	}
	for ai := range cd.Abilities {
		// An instant's or sorcery's additional cost rides on its Spell ability
		// (kgen build: cb.spellCost), not CardDef.SpellCost.
		if a := &cd.Abilities[ai]; a.Kind == kop.AbSpell && a.Cost.Flags&sacOrReturn != 0 {
			return true
		}
	}
	return false
}

// filtFeeGaps is the filters of gaps that carry a mana fee and make more than
// one unit (gorge's scriptSearch.feeGaps): a one-unit fee source nets nothing
// after its fee, so the fee-only paths price it and funding it first only
// changes the node walk. A dynamic amount is no unit count there (gorge's
// availableAmount reads only a literal one, so Three Tree City's {2}, {T} makes
// zero units), whatever it resolves to now.
func (s *State) filtFeeGaps(k *prep, gaps slotMask) (m slotMask) {
	for b := gaps; b.any(); b = b.rest() {
		sl := b.first()
		if s.filtAb(k, sl).AFlags&kop.AfManaDynamic != 0 {
			continue
		}
		if s.filtFee(k, sl) > 0 && s.filtUnits(k, sl) >= 2 {
			m = m.or(slotBit(sl))
		}
	}
	return m
}

// filtFeeOrder reports whether gorge's phased run for play a is the fee
// order, and the fee-carrying gaps it walks between the funding and the free
// gaps: a plain cast (no mode, no alternative cost) with a fee-carrying gap,
// outside phasedScope (the cast's own sacrifice or return, a hybrid-fee gap),
// where the phased run keeps its own order.
func (s *State) filtFeeOrder(k *prep, a Action, gaps slotMask) (slotMask, bool) {
	if a.Kind() != AkCast {
		return slotMask{}, false
	}
	if _, _, ok := s.playBaseCost(k, a); !ok {
		return slotMask{}, false
	}
	fee := s.filtFeeGaps(k, gaps)
	if !fee.any() || s.filtHybIn(k, gaps) || castConsumesOwn(&k.t.Cards[s.castFaceCard(k, a)]) {
		return slotMask{}, false
	}
	return fee, true
}

// feeSkip is visit's fee-order filter on a gap pass: pass 1 walks the
// fee-carrying gaps only, pass 2 the fee-free ones.
func (fs *filtSearch) feeSkip(sl Slot, pass int) bool {
	if !fs.feeOrder {
		return false
	}
	switch pass {
	case 1:
		return !fs.feeGaps.has(sl)
	case 2:
		return fs.feeGaps.has(sl)
	}
	return false
}

// feeOrderShape is passShape in the fee order: pass 0 the funding (left once
// the funding is spent, as gorge's phase loop continues to the fee gaps that
// pay their fees from it), passes 1 and 2 the fee-carrying then the fee-free
// gaps.
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap, freeOnly, skip bool) {
	if pass == 0 {
		return false, false, funded >= fs.fees
	}
	return true, false, false
}
