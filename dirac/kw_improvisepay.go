package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Improvise's planned cast (CR 702.126a, 601.2h). gorge's cast planner prices a
// plain hand cast whose only tap-paid keyword is improvise at the credited cost
// (castOfferBase's greedy artifacts), with those artifacts held back from the
// mana plan (planImprovise); the lands pay the pips an artifact cannot. A cast
// the plan cannot pay at that credit is the planner's "cost:credit" decline
// and stays the manual window's (a greedy credit is no proof of unpayable).

// improviseLifted reports whether gorge's cast planner prices p's cast of c
// (object o, fromHand) at the improvise credit: a plain hand cast with the one
// keyword, no X, and no sacrificed or tapped permanent beside the mana
// (payment_plan.go paymentPlanCastShapeDetailAltAdd). Convoke and delve keep
// "shape:contribution".
func (s *State) improviseLifted(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	if !fromHand || k.t.Cards[c].Cost.X != 0 || !s.isImprovise(k, p, c, o, fromHand) ||
		s.hasConvoke(k, p, c, o, fromHand) || s.hasCastDelve(k, p, c, o, fromHand) {
		return false
	}
	return spellAddCost(k, c).Flags&(kop.CostSac|kop.CostTapX) == 0
}

// improviseReserved is the artifacts gorge's improviseCost credits against a
// printed generic of gen (battlefield order, untapped, none already used), the
// ones the plan leaves out of its mana sources.
func (s *State) improviseReserved(k *prep, p uint8, gen uint8) slotMask {
	var used slotMask
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(SeatIx(p), s.ctrlM[SeatIx(p)].and(s.occ), ord[:], 0)
	for _, sl := range ord[:n] {
		if gen == 0 {
			break
		}
		if !s.tapPayCand(k, sl, true) {
			continue
		}
		used = used.with(sl)
		gen--
	}
	return used
}

// improviseSourcesPay reports whether p's pool and sources, less the reserved
// artifacts, pay cost (the plan's census), the restricted sources and mana
// the spell's payment may spend beside it (CR 106.6, 601.2g-h; restrFallback).
func (s *State) improviseSourcesPay(k *prep, p uint8, o Obj, res slotMask, cost *kop.ManaCost) bool {
	m := s.ctrlM[SeatIx(p)]
	s.ctrlM[SeatIx(p)] = m.andNot(res)
	var av avail
	s.availFor(k, p, &av)
	ok := av.affords(cost, pipMaskOf(cost), 0) ||
		s.restrFallback(k, p, s.objs[o].card, s.restrOriginOfObj(o), cost, &av)
	s.ctrlM[SeatIx(p)] = m
	return ok
}

// improvisePlanned classifies the mana step of spell entry e when gorge's planner
// priced its Improvise cast (build.impPlan, set at the cast). With artifacts
// announced the planned payment is re-armed on what they leave (rearmAutoPay):
// plan. With none announced beside a credit the offer's witness no longer matches
// the price (cost_changed) and the cast falls back to the manual window: manual.
// With no credit at all the witness stands at the printed cost: printed.
func (s *State) improvisePlanned(k *prep, e *stackEntry) (plan, manual, printed bool) {
	if k.gen == nil || !k.gen.anyConvoke || !s.bld().impPlan || e.kind != seSpell ||
		e.flags&(sfMayPlay|sfFlashback|sfKicked) != 0 || !s.tapPaysEntry(k, e) ||
		!s.improviseLifted(k, e.ctrl, e.card, e.obj, s.bld().origin == kop.ZoneHand) {
		return false, false, false
	}
	if s.convokePickCount() > 0 {
		return true, false, false
	}
	if s.improviseReserved(k, e.ctrl, k.cards[e.card].cost.Generic).any() {
		return false, true, false
	}
	return false, false, true
}

// improvisePlanPay pays an Improvise cast gorge's planner priced, with the
// artifacts announced. gorge runs the plan armed at the offer (priced with the
// greedy credit's artifacts held back) while it still describes the payment: the
// same generic is left to pay and no step taps an announced artifact. Otherwise
// the plan is re-armed on what the announced artifacts leave (rearmAutoPay). The
// planner reads the seat's control mask, so the held-back artifacts leave that.
// It reports that the payment is stranded and the cast reversed.
func (s *State) improvisePlanPay(k *prep, e *stackEntry, cost *kop.ManaCost, anyColor bool, saved *[len(build{}.picks)]uint8) bool {
	occ := s.occ
	hidden := s.convokeHidden()
	*cost = s.convokeApplyPicks(k, xFolded(*cost, int(e.x)))
	s.occ = occ.andNot(hidden)
	if s.bld().bfl&bflMsDone == 0 && s.paymentStranded(k, e, cost) {
		s.occ = occ
		s.releaseCostTaps(k, saved)
		s.reverseCast(k)
		return true
	}
	s.occ = occ
	si := SeatIx(e.ctrl)
	ctrl := s.ctrlM[si]
	hold := hidden
	if g := s.improviseReserved(k, e.ctrl, k.cards[e.card].cost.Generic); g.count() == hidden.count() {
		c := *cost
		t := s.fork()
		t.ctrlM[si] = ctrl.andNot(g)
		t.tapForRider(k, e.ctrl, &c, int(e.x), e.obj, anyColor)
		conflict := false
		for m := hidden; m.any(); m = m.rest() {
			if t.perms[m.first()].flags&pfTapped != 0 {
				conflict = true
			}
		}
		s.unfork()
		if !conflict {
			hold = g
		}
	}
	s.ctrlM[si] = ctrl.andNot(hold)
	s.tapForRider(k, e.ctrl, cost, int(e.x), e.obj, anyColor)
	s.ctrlM[si] = ctrl
	return false
}

// improviseRoute routes the mana step of a spell cast whose plan gorge's
// planner takes at the printed price with the tap-paid permanents held out of
// it: a target-paid convoke cast with no creature announced (state.go
// bflTargetPaid), and an Improvise cast it priced at the credited cost
// (build.impPlan, kw_improvisepay.go). Both shapes pay through the plan here;
// the shapes that fall back to the manual CR 601.2g window — convoke or delve
// beside the mana, an Improvise credit reserved but not announced (manual) —
// return not handled and manaStep's own arms pay. impPlan and bflTargetPaid
// are set exclusively (cast.go), so a manual improvise cast can never take the
// target-paid arm. It reports (handled, stranded): stranded, the payment
// stranded and the cast was already reversed (CR 733.1).
func (s *State) improviseRoute(k *prep, e *stackEntry, cost *kop.ManaCost, anyColor bool, saved *[len(build{}.picks)]uint8) (handled, stranded bool) {
	plan, manual, printed := s.improvisePlanned(k, e)
	if k.gen != nil && k.gen.anyConvoke && s.tapPaysEntry(k, e) && (s.bld().bfl&bflTargetPaid != 0 || printed) && s.convokePickCount() == 0 && !manual {
		// a target-paid convoke cast with no creature announced keeps its
		// plan at the printed cost (state.go bflTargetPaid): the planner
		// pays, the Treasure for the colour the cost owes. An Improvise cast
		// with the credit printed and no artifact beside it (printed) takes
		// the same shape.
		s.tapForRider(k, e.ctrl, cost, int(e.x), e.obj, anyColor)
		return true, false
	}
	if plan {
		// an Improvise cast gorge's planner prices: the plan pays it while it
		// still describes the payment; stranded, the cast is reversed
		return true, s.improvisePlanPay(k, e, cost, anyColor, saved)
	}
	return false, false
}
