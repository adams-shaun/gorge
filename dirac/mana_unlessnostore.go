package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// availNoStore is the availFor mode of an unless cost's affordability probe:
// gorge's unless window census (rules unlessWindowUnits) prices no storage
// battery -- its announced X (Sand Silos' "{T}, Remove X storage counters")
// is no unit the reach search can tap -- so a payer whose only source of the
// mana is a battery holding counters is offered decline alone (Mtenda Lion's
// "defending player may pay {U}" beside a charged Sand Silos). A trigger's own
// cost keeps the batteries (trigCostPotentialPays, storeSourceCounts).
const availNoStore uint16 = 4096

// unlessUnpriced: counter-cost mana ability a is a unit gorge's unless census
// (rules unlessWindowUnits) lists none for: a storage battery's announced X,
// or an any-colour production (Vivid Creek's "{T}, remove a charge counter:
// add one mana of any color"), whose colour the census withholds because the
// activation asks it (paid_mana_census.go admits a SubCounter fee only on a
// fixed production, pay.ManaSubCounterOnlyCost). With a Viridescent Bog's {1}
// funded by a Mountain, "sacrifice Child of Gaea unless you pay {G}{G}" offered
// pay only here, the second {G} coming from the counter.
func unlessUnpriced(a *kop.Ability) bool {
	return kop.StoreManaAb(a) || ctrAnyColour(a)
}

// canPayGenFor is canPayGen under skip's payment class: an unless cost (pfModes)
// reads availFor with availNoStore.
func (s *State) canPayGenFor(k *prep, p uint8, kind int, skip uint16, amt int, pips int16, f kop.FilterID) bool {
	if kind != kop.PayMana || skip&pfModes == 0 {
		return s.canPayGen(k, p, kind, amt, pips, f)
	}
	x := s.g.availX
	s.g.availX |= availNoStore
	ok := s.canPayGen(k, p, kind, amt, pips, f)
	s.g.availX = x
	return ok
}

// windowCtrOfferFor is windowCtrOffer for a window pick: an unless / ward
// census window (census) offers no storage battery (availNoStore).
func (s *State) windowCtrOfferFor(k *prep, p uint8, o Slot, decided bool, want uint8, census bool) bool {
	if census && k.ctrMana {
		if a := k.ctrAbOf(s.cardAt(o)); a != nil && unlessUnpriced(a) {
			return false
		}
	}
	return s.windowCtrOffer(k, p, o, decided, want)
}

// censusNoStore drops the storage batteries from gap set m: the unless / ward
// census's prefix search (censusPayGaps) activates none (availNoStore).
func (s *State) censusNoStore(k *prep, m slotMask) slotMask {
	if !k.ctrMana {
		return m
	}
	for b := m; b.any(); b = b.rest() {
		sl := b.first()
		if a := k.ctrAbOf(s.cardAt(sl)); a != nil && unlessUnpriced(a) {
			m = m.without(sl)
		}
	}
	return m
}
