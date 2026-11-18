package dirac

// A plain cast the autopay surface keeps because gorge's planner prices it
// with a life-only mana source (Treasonous Ogre's "Pay 3 life: Add {R}").
// The planner's census lists the ability as a last-resort alternative (a cost
// of {T} optional and PayLife: pay.PlanLastResortCostOK), so when the normal
// sources alone fall short phase 2 adds it, and the plan is a witness the
// autopay vocabulary offers (CR 601.2g, 605.1a). The planner's lethal guard
// refuses a plan whose summed life reaches the payer's life (pay/search.go
// walk: pain >= life), so a payment that would leave 0 life is no plan.
// scriptOffers drops every other plain cast only a census gap pays
// (autopayDropsPlainCast): the planner prices none of those.

// lifeGapPlanPrices reports whether the life-only gaps alone, activated in
// battlefield order, make play a affordable with the payer left above 0
// life: the plan gorge's phase 2 finds. life is p's payable life-only gaps.
func (s *State) lifeGapPlanPrices(k *prep, a Action, p uint8, life slotMask) bool {
	if !life.any() {
		return false
	}
	var order [MaxPerm]Slot
	n := s.scriptGapOrder(k, a, p, life, order[:])
	var path [MaxPerm]uint8
	var dead bool
	m, ok := s.scriptDFS(k, a, p, order[:n], slotMask{}, slotMask{}, slotMask{}, slotMask{}, slotMask{}, path[:], &dead)
	if !ok || m == 0 {
		return false
	}
	paid := 0
	for i := 0; i < m; i++ {
		ab, ok := s.lifeGapSource(k, order[i])
		if !ok {
			return false
		}
		paid += int(ab.Cost.N)
	}
	return paid < int(s.pl[SeatIx(p)].life)
}
