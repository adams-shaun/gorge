package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

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
	return s.storeCensusGapIf(k, p, false)
}

// storeCensusLoadedGap is storeCensusGap for gorge's payment-plan census
// (paymentPlanCensusOf), which skips a source that yields nothing
// (manaAbilityYieldsNothing): a storage battery holding no counters makes no
// mana, so it is no census gap there, and the madness offer's count proof
// stands beside it (CR 601.2g, 605.3a). Only a loaded battery is a gap.
func (s *State) storeCensusLoadedGap(k *prep, p uint8) bool {
	return s.storeCensusGapIf(k, p, true)
}

func (s *State) storeCensusGapIf(k *prep, p uint8, loaded bool) bool {
	if !k.ctrMana {
		return false
	}
	for b := s.scriptCtrIdle(k, p, true).and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		a := k.ctrAbOf(c)
		if !kop.StoreManaAb(a) || k.ctrSecond(c) {
			continue
		}
		if loaded && s.perms[sl].ctrN(s, int(a.Cost.Ctr)) <= 0 {
			continue
		}
		return true
	}
	return false
}

// storeCensusPays: an idle storage battery's single-colour production is an
// open supply in gorge's potential walk (it prices the battery's SubCounter<X>
// cost at no fixed count), so a keyword turn-up the other sources and that
// colour together pay is a potential play: the battery's colour pips and the
// generic part are the battery's, the rest the untapped sources'. The offer is
// that potential read; the payment re-proves the post-tap plan against the
// counters the battery actually holds (storeTurnUpTap).
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
		if s.perms[sl].ctrN(s, int(a.Cost.Ctr)) <= 0 {
			// gorge's planner announces X against the counters the battery
			// holds: an empty one funds nothing, so the turn-up is no play
			// (its plan is "insufficient") and the oracle drops it
			continue
		}
		if cost.Hyb[0] == 0 {
			// the counters it holds, with the sources, pay the cost: gorge's
			// witness is that plan, so a battery too short for the generic
			// part it would fund is no potential play either
			if s.storeTurnUpPlans(k, p, sl, a, cost) {
				return sl, true
			}
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

// storeTurnUpTap activates a battery storeCensusSource's arm lists (every
// counter removed, that many mana of its colour into the pool: gorge's
// ManaCostSubCounterStage takes X at the bound), so the planner taps the
// remaining sources against a pool the battery has funded. The offer's read
// was unbounded (storeCensusPays), so the payment re-proves the post-tap plan
// first and reports whether it activated anything: a short battery leaves
// turnFaceUp a no-op, as gorge's morphTurnUpPayableAs does.
func (s *State) storeTurnUpTap(k *prep, p uint8, cost *kop.ManaCost) bool {
	if !k.ctrMana {
		return false
	}
	for b := s.scriptCtrIdle(k, p, true).and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		a := k.ctrAbOf(s.cardAt(sl))
		if a == nil || !kop.StoreManaAb(a) || k.ctrSecond(s.cardAt(sl)) {
			continue
		}
		if !s.storeTurnUpPlans(k, p, sl, a, cost) {
			continue
		}
		s.storeGap(k, p, sl, a)
		return true
	}
	return false
}

// storeTurnUpPlans reports whether activating the battery at sl for every
// counter it holds (storeGap) leaves cost payable: the activation adds n mana
// of the battery's one colour to the pool, and the planner must fund the rest
// exactly as tapForTurnUp will. The probe is dry (pk.dry): the pool is
// simulated by adding the n units and then restored, so nothing taps. A
// battery holding no counter makes nothing (X = 0), and a hybrid pip's own
// resolution path is not this probe (turnUpPlannerPays).
func (s *State) storeTurnUpPlans(k *prep, p uint8, sl Slot, a *kop.Ability, cost *kop.ManaCost) bool {
	if cost.Hyb[0] != 0 {
		return false
	}
	n := s.perms[sl].ctrN(s, int(a.Cost.Ctr))
	if n <= 0 {
		return false
	}
	n = min(n, 255)
	col := bits.TrailingZeros8(uint8(a.ManaColors) & 0x3f)
	pl := &s.pl[SeatIx(p)]
	before := pl.pool[col]
	pl.pool[col] = uint8(min(255, int(before)+n))
	var pk planner
	pk.dry = true
	s.tapPlanNeed(k, p, cost, 0, NoObj, false, false, &pk)
	pl.pool[col] = before
	return pk.needT == 0 || pk.found
}
