package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A hybrid-fee filter (kop.HybridFilterManaAb: Fetid Heath's "{W/B}, {T}: Add
// {W}{W}, {W}{B}, or {B}{B}"), the card's second mana ability beside a plain
// {T}: Add {C}.
//
// gorge's planner census misses it (a paid gap) and cannot relax it: a hybrid
// fee has no relaxed alternative (rules paymentPlanRelaxedAlternatives), so
// the census is unrelaxable (filtRelaxable). It funds one fee like any other
// paid gap (fees), so the prefix search taps a covered source to float the
// pip before activating the filter (filtFunding). Its activation ask lists
// {C}, then the Combo's one option per colour of the amount, "Add WW" and
// "Add BB" (pay.ManaAbilityComboColours flattens it): no {W}{B} option, so
// the kernel plays the production as ManaN of one chosen colour (filtUnits,
// filtColours).

// filtHyb reports the two colours of the hybrid fee of the filter at sl, in
// the printed order gorge's payment tries them. It reads the ability itself
// (no genPrep gate: a new fixed field fails TestNoNewFixedStateFields, and
// every per-card flag bit is taken).
func (s *State) filtHyb(k *prep, sl Slot) (first, second int, ok bool) {
	if !k.gen.has(s.cardAt(sl), gfFilt) {
		return 0, 0, false
	}
	a := s.filtAb(k, sl)
	if !kop.HybridFilterManaAb(a) {
		return 0, 0, false
	}
	first, second = hybColours(a.Cost.Mana, 0)
	return first, second, true
}

// filtHybIn reports whether some filter of f has a hybrid fee.
func (s *State) filtHybIn(k *prep, f slotMask) bool {
	for ; f.any(); f = f.rest() {
		if _, _, ok := s.filtHyb(k, f.first()); ok {
			return true
		}
	}
	return false
}

// gapFaceHybIn reports whether some mana ability of a gap's face carries a
// true hybrid pip: gorge's phasedScope reads every mana ability of each gap's
// face (its priced cost's Hybrid pips), not just the filter ability — a gap's
// second mana ability can carry the fee. dirac reads the printed cost, which
// a cost static reaching mana abilities could differ from. The sentinels
// HybTwobrid and HybPhyrexian are not hybrid pips: gorge's phasedScope reads
// only Cost.Hybrid, not Twobrid, Phyrexian or HybridPhyrexian.
func (s *State) gapFaceHybIn(k *prep, f slotMask) bool {
	for ; f.any(); f = f.rest() {
		c := s.cardAt(f.first())
		for i := range k.t.Cards[c].Abilities {
			m := k.t.Cards[c].Abilities[i].Cost.Mana
			if k.t.Cards[c].Abilities[i].Kind != kop.AbMana {
				continue
			}
			for _, h := range m.Hyb {
				if h != 0 && h != kop.HybTwobrid && h != kop.HybPhyrexian {
					return true
				}
			}
		}
	}
	return false
}

// hybridPhasedScope reports whether gorge's phased run follows the plain run
// fs, which found no script, for a hybrid-fee gap (scriptSearch.phasedScope):
// a plain cast whose census gaps include a mana ability with a hybrid fee
// (Mystic Gate's "{W/U}, {T}: Add {W}{W}, {W}{U} or {U}{U}") beside a
// repeatable fee-only filter (Energy Refractor's "{2}: Add one mana of any
// color"). The phased run carries no fee-only ask prune (feeStall is the plain
// run's alone, CR 605.3a), so it walks the line that taps the funding sources,
// pays the fee-only filter, floats a colour a free tap could also make, and
// pays the cast. A search whose gaps are not filters alone stays with the plain
// run's answer (phasedFallback's scope).
func (s *State) hybridPhasedScope(k *prep, a Action, fs *filtSearch) bool {
	return fs.feeOnly && fs.pay == nil && !fs.other.any() && a.Kind() == AkCast && a.Mode() == 0 && s.gapFaceHybIn(k, fs.gaps)
}

// filtHybHeld is how much of the hybrid fee's colours the filter at sl may
// spend: p's pool and the restricted mana its activation may use.
func (s *State) filtHybHeld(k *prep, p uint8, sl Slot, a, b int) int {
	return s.filtPipHeld(k, p, sl, a) + s.filtPipHeld(k, p, sl, b)
}

// filtPayHyb pays the hybrid fee of the filter at sl: the first printed colour
// when held, else the second.
func (s *State) filtPayHyb(k *prep, p uint8, sl Slot, a, b int) {
	if s.filtPipHeld(k, p, sl, a) >= 1 {
		s.filtPayPip(k, p, sl, a)
		return
	}
	s.filtPayPip(k, p, sl, b)
}
