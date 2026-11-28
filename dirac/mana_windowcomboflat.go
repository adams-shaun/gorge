package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A finite-mask Combo source in a payment window beside another mana ability
// (Vivi Ornitier's {0} "add X mana in any combination of {U} and/or {R}" while
// it holds a granted {T}: Add {G}). gorge's "choose a mana ability" ask
// flattens the printed Combo into one symbolled option per colour
// (rules mana_activation.go activateManaFor, pay.ManaAbilityComboColours), and
// the window's answer (payexec.WindowAnswer) takes the first symbolled option
// the cost owes, else the lowest symbol: a granted ability's option has no
// symbol and is never taken beside them. The answer resolves the Combo with
// its Produced$ rewritten to that colour, so the whole amount floats in it
// and nothing taps (CR 605.3a). Alone, the source asks its allocation
// (windowComboTap's walk).

// finiteComboGrantView reports whether pc is a granted ability's view over
// card c, a finite-mask Combo source with no census ability of its own: the
// view's ability index names the grant's, so it marks none of the card's
// abilities done in the potential walk, which then admits the Combo beside it
// (gorge potentialManaFor counts every ability of a source, CR 605.3a).
func (k *prep) finiteComboGrantView(c CardID, pc *pcard) bool {
	return k.gen != nil && k.isComboAny(c) && pc != &k.cards[c] && k.cards[c].manaAb < 0 && k.comboAnyAb(c).ManaColors != kop.ColAny
}

// windowFlatCombo reports whether the source at sl is such a flattened Combo
// now: live and offered beside a granted ability. It is listed at amount zero
// too (a power-0 Vivi): the answer then taps nothing and adds nothing.
func (s *State) windowFlatCombo(k *prep, p uint8, sl Slot) bool {
	return k.gen != nil && k.gen.anyComboAny && s.ctrlM[SeatIx(p)].has(sl) && s.caFlattened(k, sl) &&
		s.caGapLive(k, p, sl, k.comboAnyAb(s.cardAt(sl))) &&
		(s.grantTapOK(k, sl) || s.grantSacOK(k, sl))
}

// failedPayCombo is tapOneForMana's arm for a Combo or reflected Amount$
// source: gorge's failed ward payment activates it as any listed source,
// answering the lowest colour, since the window's ask carries no cost. It
// reports whether the source was activated. One whose ability taps nothing
// (Vivi's {0}) is then still listed for a granted {T}.
func (s *State) failedPayCombo(k *prep, p uint8, sl Slot) bool {
	if !s.windowFlatCombo(k, p, sl) && !s.caGaps(k, p).has(sl) {
		return false
	}
	var none kop.ManaCost
	return s.windowGapTap(k, p, sl, s.cardAt(sl), &none, &none, false, true)
}

// windowFlatComboPick is windowPick's arm for a flattened Combo (windowGapArm
// answers it): hit when the window lists it, skip when the clamp keeps neither
// its colours nor a granted ability's (gorge paymentSourceUseful reads every
// ability).
func (s *State) windowFlatComboPick(k *prep, p uint8, sl Slot, decided bool, want uint8, census bool) (hit, skip bool) {
	if census || !s.windowFlatCombo(k, p, sl) {
		return false, false
	}
	if decided && (uint8(s.caMask(k, sl))|s.windowGrantMask(k, sl))&want == 0 {
		return false, true
	}
	return true, false
}

// windowOwedPick is the picks windowPick lists ahead of a plain usable source,
// in this order: the unless census's fixed counter-fee source
// (ctrCensusOffer); the source's owed later sac-tap ability, which the
// window's answer takes (mana_sactapwin.go); a finite Combo beside another
// ability. hit lists sl, skip drops it.
func (s *State) windowOwedPick(k *prep, p uint8, sl Slot, useful *kop.ManaCost, decided bool, want uint8, census bool) (hit, skip bool) {
	if census && s.ctrCensusOffer(k, p, sl) {
		return true, false
	}
	if _, ok := s.windowSacTapOwed(k, p, sl, useful); ok {
		return true, false
	}
	return s.windowFlatComboPick(k, p, sl, decided, want, census)
}

// windowFlatCol is the colour the window answers a flattened Combo's options
// with: the first listed colour the cost still owes, else the lowest. A cast's
// window carries no payment, so it names an owed pip only once the generic is
// covered (windowComboFlatCol); a window with no clamp and nothing owed (an
// unless or ward cost) takes the lowest.
func (s *State) windowFlatCol(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, noOwed bool) int {
	mask := uint8(s.caMask(k, sl))
	var owed kop.ManaCost
	switch {
	case !noOwed:
		s.windowOwedCost(k, p, cost, &owed)
	case clamp:
		if _, decided := s.windowUseful(k, p, useful); decided {
			owed = *useful
		}
	}
	var order [kop.NumManaKinds]uint8
	for _, c := range order[:s.stepColours(k, sl, -1, mask, &order)] {
		if owed.Pips[c] > s.pl[SeatIx(p)].pool[c] {
			return int(c)
		}
	}
	return bits.TrailingZeros8(mask)
}
