package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// actRestrDisjoint is costManaDisjoint for an activation whose mana only the
// restricted sources (or the floating restricted mana) it may spend pay
// (abilityAffords: the plain census av alone fell short). The sacrifice or
// tap-another part's candidates are kept apart from the mana AFTER those
// sources are counted, as gorge's planner prices the composed cost from every
// source the payment may spend (planComposedCost, CR 601.2g-h, 106.6): Base
// Camp's any-colour mana pays the {U} of Jalira's "{2}{U}, {T}, Sacrifice
// another creature" beside two Swamps, and the Eldrazi Scion that is the only
// sacrifice candidate (it sacrifices for {C} too) is held back from that mana.
// Checking the part against the plain census, which holds no restricted
// source, refused the activation. skip is the source whose own {T} the
// activation takes (restrSrcAvail), pm the cost's pip mask.
//
//go:noinline
func (s *State) actRestrDisjoint(k *prep, sl Slot, a *kop.Ability, cost *kop.ManaCost, av *avail, skip Slot, pm uint8) bool {
	if s.costManaDisjoint(k, sl, a, av, pm) {
		return true // the plain census's answer stands: restricted mana only adds units
	}
	p := s.perms[sl].ctrl
	d := payDesc{class: pdAct, card: s.cardAt(sl), src: sl, instant: a.Cost.Flags&kop.CostSorcery == 0}
	t := *av
	if m := s.restrPoolAct(k, p, &d); m != 0 {
		s.restrAvailAdd(p, m, &t)
	}
	if k.restrSrc && k.gen != nil {
		t, _, _ = s.restrSrcAvail(k, p, d, &t, skip, nil, 0)
	}
	t.ungr = av.ungr
	return s.costPartDisjointD(k, p, sl, &a.Cost, cost, &t, pm, &d)
}
