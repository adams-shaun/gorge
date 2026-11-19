package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// tapForTry is tapForRider for a cost the caller may find unpayable: it taps
// the plan and reports true, or taps nothing and reports false when the
// planner has none (tapForRider panics there, a kernel defect for a cost the
// offer priced). Only a cost the offer could not price yet, as a per-target
// raise charged after the targets (cost_raise.go), and an Improvise payment
// re-planned on the artifacts the player announced (improvisePlanPay), use it.
func (s *State) tapForTry(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, anyColor bool) bool {
	var pk planner
	pk.try = true
	s.tapForPK(k, p, cost, x, exclude, false, anyColor, &pk)
	return pk.needT == 0 || pk.found
}

// tapForSpellPlain taps the plain cast's sources for cost and reports true when
// it reversed the cast instead. A card with "costs {1} more for each target
// beyond the first" (Fireball) has the targets chosen after the X was announced
// raise the price past what the sources make: CR 601.2c lets the player choose
// any number of targets and 601.2h finds the cost unpayable afterwards, so the
// cast is reversed (CR 733.1) where the plan would find none. Every other cast
// keeps tapForRider and its panic, for a cost the offer priced.
func (s *State) tapForSpellPlain(k *prep, e *stackEntry, cost *kop.ManaCost, anyColor bool, saved *[len(build{}.picks)]uint8) bool {
	// the planner arms tap for what the admitted restricted mana leaves
	// (pay.PlanPool); the caller hands us the unshortened cost, because the
	// window fallback below must not take the shortened one: its settle
	// (windowSpellSettled) discounts the admitted floating batches itself, as
	// the seAbility branch's paymentStranded does, and handed the shortened
	// cost it would count that mana twice and tap short a payment the window
	// funds (as the k.phyMana arm keeps phyPay unshortened).
	short := s.restrAdmShortCost(k, e, *cost)
	if s.buildIsTb(k) {
		// A planned twobrid cast (CR 107.4e): the cast planner resolves the
		// announced faces (planTwobridCost) and prices the printed census. A
		// cost that census cannot fund -- a filter's fee paid from the floated
		// pool, a granted source -- has no witness, and gorge's plan falls
		// back to its manual CR 601.2g window. Plan first (the try taps only
		// when it finds a witness, so a planner-priceable cast keeps the
		// planner's ObjID-ordered payment), then pay that window rather than
		// panicking on the missing plan.
		if s.tapForTry(k, e.ctrl, &short, int(e.x), e.obj, anyColor) {
			return false
		}
		if !s.tapForHyb(k, e.ctrl, cost, int(e.x), e.obj, anyColor) {
			return true // stranded: the cast was reversed
		}
		return false
	}
	if s.raisePerTargetCharge(e.card, e.ctrl) == 0 {
		s.tapForRider(k, e.ctrl, &short, int(e.x), e.obj, anyColor)
		return false
	}
	if s.tapForTry(k, e.ctrl, &short, int(e.x), e.obj, anyColor) {
		return false
	}
	// the planner has no witness for the taxed cost: the CR 601.2g window pays it when its sources do
	// (a filter funded by the floating mana, castWindowReachable), and only then is the cast reversed
	c := xFolded(*cost, int(e.x))
	if ok, _ := s.windowTapUseful(k, e.ctrl, &c, &c, true, anyColor, s.bld().pipN == 0, false); ok || s.winPaused() {
		return false
	}
	s.releaseCostTaps(k, saved)
	s.reverseCast(k)
	return true
}
