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
// it reversed the cast instead. The offer can price a cost the planner's census
// does not witness: an X announced against the CR 601.2g window (availXWindow)
// and a first-only filter's fee the window pays (filtMaxX, CR 605.3b), or a
// per-target raise charged after the targets (CR 601.2c, 601.2h). Gorge leaves
// an announced X to that window (pay.PlanCostDetail "cost:x"), so the planner
// is asked first (an ordinary cost keeps its planner witness) and the window
// pays only when it has none. A payment neither reaches is reversed (CR 733.1)
// rather than panicking.
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
		if !s.tbPoolOffer(k, e, short, anyColor) && s.tapForTry(k, e.ctrl, &short, int(e.x), e.obj, anyColor) {
			return false
		}
		if !s.tapForHyb(k, e.ctrl, cost, int(e.x), e.obj, anyColor) {
			return true // stranded: the cast was reversed
		}
		return false
	}
	if s.tapForTry(k, e.ctrl, &short, int(e.x), e.obj, anyColor) {
		return false
	}
	// the planner has no witness for the cost: the CR 601.2g window pays it when its sources do
	// (a filter funded by the floating mana, castWindowReachable), and only then is the cast reversed
	c := xFolded(*cost, int(e.x))
	if ok, _ := s.windowTapUseful(k, e.ctrl, &c, &c, true, anyColor, s.bld().pipN == 0, false); ok || s.winPaused() {
		return false
	}
	s.releaseCostTaps(k, saved)
	s.reverseCast(k)
	return true
}

// tbPoolOffer reports whether gorge offered the planned twobrid cast being
// built as its legacy pool cast and the caster announced other faces. Gorge's
// planTwobridCost tries every face assignment in mask order (bit i set: pip i's
// two generic; colour faces first) and keeps the first that pays; when the
// floating pool alone pays that assignment the offer carries no payment plan
// (the legacy cast option), so an announcement of other faces is paid through
// gorge's manual CR 601.2g window in battlefield order, not a fresh planner
// witness (CR 107.4e, 601.2b, 601.2g): Magmablood Archaic's {2/R}{2/R}{2/R}
// over a floating {R}{R}{R}, announced {2}{R}{R}, taps the first land for the
// last generic. cost is the resolved cost the planner would pay; a cost whose
// faces cannot be swapped (a Phyrexian pip, a generic part a reduction ate)
// keeps the planner.
func (s *State) tbPoolOffer(k *prep, e *stackEntry, cost kop.ManaCost, anyColor bool) bool {
	base := s.buildCost0(k)
	n := base.TbN()
	if base.Phyrexian() || n == 0 || int(s.bld().pipN) != n {
		return false
	}
	ann := int(s.bld().pipC[0]) & (1<<n - 1)
	for m := 0; m < 1<<n; m++ {
		c := cost
		for i := 0; i < n; i++ {
			a, b := ann>>i&1, m>>i&1
			if a == b {
				continue
			}
			col := base.TbCol(i)
			if a == 1 { // announced two generic, this assignment the colour
				if c.Generic < 2 {
					return false
				}
				c.Generic -= 2
				c.Pips[col]++
				continue
			}
			if c.Pips[col] == 0 {
				return false
			}
			c.Pips[col]--
			c.Generic += 2
		}
		var pk planner
		pk.try, pk.dry = true, true
		s.tapForPK(k, e.ctrl, &c, int(e.x), e.obj, false, anyColor, &pk)
		if pk.needT == 0 || pk.found {
			return pk.needT == 0 && m != ann
		}
	}
	return false
}
