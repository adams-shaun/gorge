package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// storeCensusGap reports whether p holds an activatable storage battery
// (Icatian Store's "{T}, Remove any number of storage counters: Add {W} for
// each"), an idle one holding no counters included: gorge's window census
// admits its ability (X is announced, so any count pays the cost) but prices no
// SubCounter<X> cost shape (castWindowSourceExcluded's paidCostShape), so the
// census is incomplete and a pool-short target menu keeps its candidates
// (affordableTargetCandidates' fallback, CR 601.2c, 601.2h). A battery the
// build committed (withheld from s.occ) is no census source. A battery with a
// plain first mana ability beside it (ctrSecond: Mage-Ring Network's {T}: Add
// {C}) is no gap: gorge prices that plain unit, so castWindowCensusComplete's
// priced arm never reaches the exclusion.
func (s *State) storeCensusGap(k *prep, p uint8) bool {
	if !k.ctrMana {
		return false
	}
	for b := s.scriptCtrIdle(k, p, true).and(s.occ); b.any(); b = b.rest() {
		c := s.cardAt(b.first())
		if kop.StoreManaAb(k.ctrAbOf(c)) && !k.ctrSecond(c) {
			return true
		}
	}
	return false
}

// storeCensusPays: an idle storage battery's single-colour production is an
// open supply in gorge's potential walk (it prices the battery's SubCounter<X>
// cost at no fixed count), so a keyword turn-up the other sources and that
// colour together pay is a potential play: the battery's colour pips and the
// generic part are the battery's, the rest the untapped sources'.
func (s *State) storeCensusPays(k *prep, p uint8, cost *kop.ManaCost) bool {
	_, ok := s.storeCensusSource(k, p, cost)
	return ok
}

// storeCensusSource is storeCensusPays' first battery.
func (s *State) storeCensusSource(k *prep, p uint8, cost *kop.ManaCost) (Slot, bool) {
	if !k.ctrMana {
		return NoSlot, false
	}
	for b := s.scriptCtrIdle(k, p, true).and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		a := k.ctrAbOf(s.cardAt(sl))
		if a == nil || !kop.StoreManaAb(a) || k.ctrSecond(s.cardAt(sl)) {
			continue
		}
		rest := *cost
		rest.Generic = 0
		for i := range rest.Pips {
			if a.ManaColors&(1<<i) != 0 {
				rest.Pips[i] = 0
			}
		}
		if s.sourcesPay(k, p, &rest) {
			return sl, true
		}
	}
	return NoSlot, false
}

// storeTurnUpTap activates the battery storeCensusSource names (every counter
// removed, that many mana of its colour into the pool: gorge's
// ManaCostSubCounterStage takes X at the bound), so the planner taps the
// remaining sources against a pool the battery has funded.
func (s *State) storeTurnUpTap(k *prep, p uint8, cost *kop.ManaCost) {
	if sl, ok := s.storeCensusSource(k, p, cost); ok {
		s.storeGap(k, p, sl, k.ctrAbOf(s.cardAt(sl)))
	}
}
