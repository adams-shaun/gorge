package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A fee-only first mana ability (kop.FeeOnlyManaAb): Nomadic Elf's "{1}{G}:
// Add one mana of any color", Orochi Leafcaller's "{G}: ...", Agent of
// Stromgald's "{R}: Add {B}", Energy Refractor's "{2}: ...". It has no {T} and
// no activation limit, so it is repeatable: nothing marks a use (CR 302.6
// does not bind it either, a tapped or summoning-sick source still activates),
// which matches gorge's scriptStateKey, whose dedup keys on the pool, the
// tapped bits and the counters alone.
//
// gorge's census (potential_plan.go paymentPlanCensusOf) lists it as a gap
// when the full PotentialMana fixpoint pays its fee, own production included,
// and paymentPlanRelaxedAlternatives gives it no relaxed alternative (it is
// repeatable and not a bounded tap part). The census is then unrelaxable and
// unbounded, its funding fee x potentialScriptNodes, and every insufficient
// play among the first potentialScriptPlays gets the prefix search.
//
// Not modelled: gorge's `!out.relaxable && !bounded` skip
// (potential_plan.go paymentPlanCensusOf), which zeroes the funding when an
// earlier gap in zone order already cleared both flags.

// filtRepeat: the filter at sl is a repeatable fee-only one.
func (s *State) filtRepeat(k *prep, sl Slot) bool {
	return k.gen.anyFiltRepeat && kop.FeeOnlyManaAb(s.filtAb(k, sl))
}

// filtRepeatIn: some filter of f is a repeatable fee-only one.
func (s *State) filtRepeatIn(k *prep, f slotMask) bool {
	if !k.gen.anyFiltRepeat {
		return false
	}
	for ; f.any(); f = f.rest() {
		if s.filtRepeat(k, f.first()) {
			return true
		}
	}
	return false
}

// filtRepeatCost is the fee of the repeatable filter at sl, after the
// activated-ability cost reducers (CR 601.2f, 602.2b).
func (s *State) filtRepeatCost(k *prep, sl Slot) kop.ManaCost {
	a := s.filtAb(k, sl)
	if k.gen.anyAbReduce {
		return s.abReduced(k, a, sl, s.perms[sl].ctrl, a.Cost.Mana)
	}
	return a.Cost.Mana
}

// filtRepeatPip is the one coloured pip of the repeatable filter at sl's fee
// (a fee of generic mana alone has none). The pip is paid first, then the
// generic part, as gorge's ResolveMana does (pay/solve.go).
func (s *State) filtRepeatPip(k *prep, sl Slot) (col int, ok bool) {
	if !s.filtRepeat(k, sl) {
		return 0, false
	}
	for c, n := range s.filtAb(k, sl).Cost.Mana.Pips {
		if n != 0 {
			return c, true
		}
	}
	return 0, false
}

// filtRepeatFee is the whole fee of the repeatable filter at sl: its generic
// part after the reducers plus its pip.
func (s *State) filtRepeatFee(k *prep, sl Slot) int {
	n := int(s.filtRepeatCost(k, sl).Generic)
	if _, ok := s.filtRepeatPip(k, sl); ok {
		n++
	}
	return n
}

// filtFixed: the first-only filter at sl makes one fixed colour (the repeatable
// fee-only one, or a {pip}, {T} one: Apprentice Wizard's {U}, {T}: Add {C}{C}{C}),
// so its activation asks nothing (a one-ability source resolves directly).
func (s *State) filtFixed(k *prep, sl Slot) bool {
	if !s.filtRepeat(k, sl) {
		if _, pip := s.filtPip(k, sl); !pip || !s.filtFirstOnly(k, sl) {
			return false
		}
	}
	a := s.filtAb(k, sl)
	return a.AFlags == 0 && a.ManaColors != kop.ColAny
}

// filtRepeatPotential: the potential pool (gorge's full PotentialMana
// fixpoint, a repeatable source's own production included) pays the fee of
// the repeatable filter at sl.
func (s *State) filtRepeatPotential(k *prep, p uint8, sl Slot, av *avail, have *bool) bool {
	if !*have {
		s.availPotential(k, p, av)
		*have = true
	}
	c := s.filtRepeatCost(k, sl)
	return av.affords(&c, pipMaskOf(&c), 0)
}

// filtFunding is the funding the prefix search may activate for the filter
// gaps f: a repeatable gap's census is unbounded, so the funding never binds
// (any value of at least potentialScriptNodes is gorge's fee x 160: a funding
// step costs a node); otherwise the fees summed, none for an unrelaxable
// census.
func (s *State) filtFunding(k *prep, p uint8, f slotMask) int {
	if s.filtRepeatIn(k, f) {
		return potentialScriptNodes
	}
	n := 0
	if s.filtFunded(k, p, f) {
		for ; f.any(); f = f.rest() {
			n += s.filtFee(k, f.first())
		}
	}
	return n
}
