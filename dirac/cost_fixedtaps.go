package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A spell whose own Cost$ adds nothing to its printed mana cost but a fixed-count
// tapXType part (Fear of Exposure's "tap two untapped creatures and/or lands",
// Guardian of the Great Door's four) is declined by gorge's cast planner
// ("shape:additional_cost", planDeclinesAddCost), but its potential planner prices
// it from the composed cost (rules potential_plan.go spellExtrasAreFixedTaps,
// planComposedCost): each assignment of candidates is withheld from the mana plan,
// so the play is priced only when the plan leaves N candidates untapped. That
// verdict is a proof, so a board whose granted mana (Enduring Vitality's
// creatures) is a census gap runs the prefix search on it: the granted sources
// are tapped in battlefield order until the composed cost prices (CR 601.2g-h).

// spellFixedTapsOnly mirrors gorge's spellExtrasAreFixedTaps for card c's plain
// cast: the spell's additional cost is one fixed-count tap of other permanents.
func spellFixedTapsOnly(k *prep, c CardID) bool {
	if k.t == nil {
		return false
	}
	cd := &k.t.Cards[c]
	if cd.HasAltAdd || cd.AltAddOne || cd.HasModeCost || cd.Cost.X != 0 || cd.Cost.Hyb[0] != 0 {
		return false
	}
	ac := spellAddCost(k, c)
	return ac.Flags == kop.CostTapX && ac.N > 0 && ac.Mana == (kop.ManaCost{})
}

// fixedTapsCast reports a plain cast a of a spellFixedTapsOnly card.
func (s *State) fixedTapsCast(k *prep, a Action) bool {
	return a.Kind() == AkCast && a.Mode() == 0 && a.Target() == 0 && spellFixedTapsOnly(k, s.objs[a.Source()].card)
}

// affordsUngrantedReserving is affordsUngrantedFor with cost part c's tap or
// sacrifice candidates kept apart from the mana (gorge planReserving): some
// assignment of them leaves a mana plan from the printed sources. src is the
// activation's source (NoSlot for a spell).
func (s *State) affordsUngrantedReserving(k *prep, p uint8, cost *kop.ManaCost, ex Slot, d payDesc, c *kop.Cost, src Slot) bool {
	var av avail
	var ms modalSet
	s.ungrAvail(k, p, ex, &av, &ms)
	pm := pipMaskOf(cost)
	for m := 0; m < 1<<ms.n; m++ {
		b := ms.modes(av, m)
		if b.affords(cost, pm, 0) && s.costPartDisjoint(k, p, src, c, cost, &b, pm) {
			return true
		}
		if k.restrSrc && d.class != pdNone && s.restrSrcAffords(k, p&1, d, cost, &b, ex, 0) {
			// restricted mana the plain avail cannot show: keep the old answer
			return true
		}
	}
	return false
}
