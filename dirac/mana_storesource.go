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
// and returns the walk's open flag: the trigger pay gate's descriptor (subBound)
// adds one mana of the battery's colour per counter, every other payment (the
// numTimes and X asks, the window's gate, a cast) keeps gorge's unbounded read, in the battery's one produced colour alone (CR 107.4a).
func (s *State) storePotential(open bool, sl Slot, ab *kop.Ability, d *payDesc, add func(Slot, uint8, uint8)) bool {
	if d == nil || !d.subBound {
		add(sl, uint8(ab.ManaColors), potentialOpenUnits)
		return open
	}
	if n := s.perms[sl].ctrN(s, int(ab.Cost.Ctr)); n > 0 {
		add(sl, uint8(ab.ManaColors), uint8(min(n, 255)))
	}
	return open
}

// storeBatteryPotential is availPotentialFor's storage-battery arm (kept out
// of the fixpoint's body): a battery's announced X (Count$xPaid) is
// indeterminate to gorge's potentialAmount, which folds it unbounded in every
// colour (potentialUnbounded) for the view and a cast. A trigger's own cost
// reads it at the counters it can remove, one mana each (rules
// potential_subcounter.go: pay ManaSubCounterXBound). The census (cen.on)
// reads the battery at the counters it can remove, one mana each, never the
// unbounded fold the over-bound pool keeps: gorge's relaxed census bounds a
// gap source at its natural amount, so a battery holding no counter makes
// nothing there (Dwarven Hold beside a kicked Flourishing Strike). Returns
// whether the ability was a battery the walk consumed (the caller skips it).
func (s *State) storeBatteryPotential(sl Slot, ab *kop.Ability, d *payDesc, cen *potCensus, drop uint8, addAb, fold func(Slot, uint8, uint8), open *bool) bool {
	if !kop.StoreManaAb(ab) {
		return false
	}
	if cen.on {
		// a battery that taps beside the source's plain {T}: {C}
		// (Mage-Ring Network) is the same tap, so its counters
		// replace that unit's one mana instead of adding to it
		if n := s.perms[sl].ctrN(s, int(ab.Cost.Ctr)); n > 0 {
			cen.recStore(sl, ab, n, drop)
		}
		*open = s.storePotential(*open, sl, ab, d, fold)
		return true
	}
	*open = s.storePotential(*open, sl, ab, d, addAb)
	return true
}
