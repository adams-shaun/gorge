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

// storePotential folds the storage battery at sl into availPotentialFor's walk
// and returns the walk's open flag: a trigger's own cost (pdOther) adds one mana
// of the battery's colour per counter, every other payment keeps gorge's
// unbounded read (open).
func (s *State) storePotential(open bool, sl Slot, ab *kop.Ability, d *payDesc, add func(Slot, uint8, uint8)) bool {
	if d == nil || d.class != pdOther {
		return true
	}
	if n := s.perms[sl].ctrN(s, int(ab.Cost.Ctr)); n > 0 {
		add(sl, uint8(ab.ManaColors), uint8(min(n, 255)))
	}
	return open
}
