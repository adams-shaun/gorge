package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// storeSourceCounts reports whether the untapped storage battery at sl (Icatian
// Store, Dwarven Hold: a census gap, so no manaAb) holding a counter is a mana
// source for a trigger's own cost. Gorge's untappedManaSourceFor counts it and
// its potential walk folds in one mana per counter (potential_subcounter.go),
// so a trigger's "you may pay {W}{W}" beside a two-counter Store is offered
// "pay"; with fewer counters the exact potential refuses it (CR 118.3).
func (s *State) storeSourceCounts(k *prep, p uint8, sl Slot) bool {
	a := k.ctrAbOf(s.cardAt(sl))
	if a == nil || !kop.StoreManaAb(a) || s.perms[sl].ctrN(s, int(a.Cost.Ctr)) <= 0 {
		return false
	}
	return s.potentialAdmits(k, p, sl, int(k.ctrAb[s.cardAt(sl)]), a, nil)
}
