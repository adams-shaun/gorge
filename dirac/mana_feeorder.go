package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorge's fee-order run (engine/rules/potential_script.go: scriptSearch.feeOrder).
// A plain cast whose census gaps hold a filter that makes two or more mana for a
// mana fee (Baxter Building's {4},{T}: four mana in any combination; a two-colour
// "{1},{T}: Add {W} or {B}") is, when the plain run finds no script, searched
// again in the fee order: the covered funding sources first, so the fee floats
// (CR 601.2g-h), then the fee-carrying gaps, then the other gaps. The plain order
// walks the filter's free {C} answer first and spends its whole node budget on
// that dead subtree when a source beside it asks (Castle Doom's {C} or any-colour
// artifact mana) before it reaches the filter's own answer.

// gapScope narrows the gaps one visit pass walks.
type gapScope uint8

const (
	gapAll      gapScope = iota // every gap
	gapFreeOnly                 // the gaps that tap for free (the phased run's first pass)
	gapFeeOnly                  // the gaps whose filter makes two or more mana for a fee (the fee order's second pass)
	gapNoFee                    // the other gaps (the fee order's third pass)
)

// filtFeeGap: the filter at sl is a fee gap (gorge's potentialPrefixScript
// feeGaps): a mana ability with a mana cost making at least two units. A
// dynamic amount counts none (availableAmount), an any-colour production its
// amount, a fixed or chosen set of colours its amount for each colour.
func (s *State) filtFeeGap(k *prep, sl Slot) bool {
	a := s.filtAb(k, sl)
	if a.AFlags&kop.AfManaDynamic != 0 || a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 {
		return false
	}
	n := int(max(a.ManaN, 1))
	switch {
	case a.ManaEach:
		n = s.filtUnits(k, sl)
	case a.ManaColors != kop.ColAny:
		n *= bits.OnesCount8(uint8(a.ManaColors) & 0x3f)
	}
	return n >= 2
}

// feeOrderShape is passShape under the fee order: pass 0 the funding sources
// (skipped once the funding floated: the fee gaps that follow pay from it),
// pass 1 the fee gaps, pass 2 the other gaps; pass 3 is visit's life pass.
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap bool, scope gapScope, skip bool) {
	switch pass {
	case 0:
		return false, gapAll, funded >= fs.fees
	case 1:
		return true, gapFeeOnly, false
	}
	return true, gapNoFee, false
}

// outOfScope: the gap at sl is not this pass's (scope).
func (fs *filtSearch) outOfScope(k *prep, t *State, sl Slot, scope gapScope) bool {
	switch scope {
	case gapFreeOnly:
		return !t.canTapForMana(k, sl) // a fee-carrying gap is the phased run's third pass
	case gapFeeOnly:
		return !t.filtFeeGap(k, sl)
	case gapNoFee:
		return t.filtFeeGap(k, sl)
	}
	return false
}

// castSacOrReturn: the cast a's own cost sacrifices or returns a permanent
// (CR 601.2b), gorge's phasedScope first arm.
func (s *State) castSacOrReturn(k *prep, a Action) bool {
	cd := &k.t.Cards[s.castFaceCard(k, a)]
	const sacOrReturn = kop.CostSac | kop.CostSacSelf | kop.CostReturn | kop.CostReturnSelf
	if cd.SpellCost.Flags&sacOrReturn != 0 || cd.AltAddCost.Flags&sacOrReturn != 0 {
		return true
	}
	for ai := range cd.Abilities {
		// An instant's or sorcery's additional cost rides on its Spell ability
		// (kgen build: cb.spellCost), not CardDef.SpellCost.
		if ab := &cd.Abilities[ai]; ab.Kind == kop.AbSpell && ab.Cost.Flags&sacOrReturn != 0 {
			return true
		}
	}
	return false
}

// feeOrderScope: gorge searches play a in the fee order when the plain run
// finds nothing (potentialPrefixScript: a plain cast of the printed cost, a
// fee gap among the census gaps, and no sacrifice, return or hybrid fee, which
// keep the phased order). A board with a gap that is no filter keeps the older
// scripts' logic.
func (fs *filtSearch) feeOrderScope(k *prep, s *State, a Action) bool {
	if a.Kind() != AkCast || a.Mode() != 0 || fs.other.any() || fs.pay != nil || s.filtHybIn(k, fs.gaps) || s.castSacOrReturn(k, a) {
		return false
	}
	for b := fs.gaps; b.any(); b = b.rest() {
		if s.filtFeeGap(k, b.first()) {
			return true
		}
	}
	return false
}
