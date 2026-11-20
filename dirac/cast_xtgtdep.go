package dirac

// xTgtDepDeclined: gorge's potential planner prices p's plain {X} cast of hand
// card o as "unsupported cost:x" -- no witness, and no census fallback or prefix
// search either -- when the cast is target-priced (spellTgtDep) and its printed
// sources and pool cannot pay it at the smallest announcement. potentialModeCastPlan
// plans the cast at that X first; its "insufficient" there sends a target-priced
// cast to potentialTargetPricedPlan, whose cost keeps the unfolded {X}
// (pay.PlanCostDetail "cost:x"). A mana ability only a grant adds (Song of
// Freyalise's creature mana beside Esior's raise on Blaze's {X}{R}) is no
// census source, so the cast is never offered (CR 601.2b, 601.2f).
func (s *State) xTgtDepDeclined(k *prep, p uint8, c CardID, o Obj) bool {
	if !s.spellTgtDep(k, p, c, o) {
		return false
	}
	cd := &k.t.Cards[c]
	cost := s.featCost(k, p, c, o, k.cards[c].cost).AtX(costXMin(&cd.SpellCost))
	return !s.ungrantedPaysNoDH(k, p, c, o, &cost)
}
