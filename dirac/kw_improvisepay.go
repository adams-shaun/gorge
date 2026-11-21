package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Improvise's planned cast (CR 702.126a, 601.2h). gorge's cast planner prices a
// plain hand cast whose only tap-paid keyword is improvise at the credited cost
// (castOfferBase's greedy artifacts), with those artifacts held back from the
// mana plan (planImprovise); the lands pay the pips an artifact cannot. A cast
// the plan cannot pay at that credit is the planner's "cost:credit" decline
// and stays the manual window's (a greedy credit is no proof of unpayable).

// improviseLifted reports whether gorge's cast planner prices p's cast of c
// (object o, fromHand) at the improvise credit: a plain hand or command-zone
// (CR 903.8; the planner's scope is "" for both) cast with the one
// keyword, no X, and no sacrificed or tapped permanent beside the mana
// (payment_plan.go paymentPlanCastShapeDetailAltAdd). Convoke and delve keep
// "shape:contribution".
func (s *State) improviseLifted(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	return s.improviseLiftedFrom(k, p, c, o, fromHand, fromHand)
}

// improviseLiftedFrom is improviseLifted with plain set for a hand or
// command-zone origin: the keyword grants still read fromHand.
func (s *State) improviseLiftedFrom(k *prep, p uint8, c CardID, o Obj, fromHand, plain bool) bool {
	if !plain || k.t.Cards[c].Cost.X != 0 || !s.isImprovise(k, p, c, o, fromHand) ||
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
// The census holds no granted mana ability, so a price the credit's artifacts
// were withheld for that only a grant pays is the planner's "cost:credit"
// decline too (a cast crediting no artifact is planned plainly and its prefix
// search taps the grant: improviseAloneCast).
func (s *State) improviseSourcesPay(k *prep, p uint8, o Obj, res slotMask, cost *kop.ManaCost) bool {
	m := s.ctrlM[SeatIx(p)]
	s.ctrlM[SeatIx(p)] = m.andNot(res)
	var av avail
	s.availFor(k, p, &av)
	ok := av.affords(cost, pipMaskOf(cost), 0) ||
		s.restrFallback(k, p, s.objs[o].card, s.restrOriginOfObj(o), cost, &av)
	if ok && res.any() && k.gen != nil && len(k.gen.grantPC) != 0 {
		// the planner's census holds no granted mana ability (pay/board.go
		// PaymentAbility): a credited price only a grant pays is its
		// "cost:credit" decline, the prefix search's (potentialCensusNeeded)
		ok = s.affordsUngrantedCast(k, p, cost, payDesc{class: pdSpell, card: s.objs[o].card, src: NoSlot, og: s.restrOriginOfObj(o)})
	}
	s.ctrlM[SeatIx(p)] = m
	return ok
}

// improvisePlanned classifies the mana step of spell entry e when gorge's planner
// priced its Improvise cast (build.impPlan, set at the cast). With artifacts
// announced the planned payment is re-armed on what they leave (rearmAutoPay):
// plan. With none announced beside a credit the cast is the cost_changed
// fallback (manual: the manual window) only when the unannounced credit lowered
// the composed price; when the credited price still equals the payment's
// composed cost (the statics already zeroed the generic the credit covered,
// CR 601.2f after CR 702.126a) gorge's paymentPlanCheck re-derives the plan's
// cost from the announcements, matches it, and keeps the offer's witness, so
// the planned payment runs with the credit's artifacts held back: plan. With no
// credit at all the witness stands at the printed cost: printed.
func (s *State) improvisePlanned(k *prep, e *stackEntry) (plan, manual, printed bool) {
	if k.gen == nil || !k.gen.anyConvoke || !s.bld().impPlan || e.kind != seSpell ||
		e.flags&(sfMayPlay|sfFlashback|sfKicked) != 0 || !s.tapPaysEntry(k, e) ||
		!s.improviseLiftedFrom(k, e.ctrl, e.card, e.obj, s.bld().origin == kop.ZoneHand, plainCastZone(s.bld().origin)) {
		return false, false, false
	}
	if s.convokePickCount() > 0 {
		return true, false, false
	}
	if s.improviseReserved(k, e.ctrl, k.cards[e.card].cost.Generic).any() {
		if DebugPlannerFlexCensus || s.improviseCreditPrice(k, e, true) != s.improviseCreditPrice(k, e, false) {
			return false, true, false
		}
		return true, false, false
	}
	return false, false, true
}

// improviseCreditPrice is the offer's credited price composed the way
// plannedCastCostOf composes it (the credit before the statics, CR 702.126a
// then 601.2f): the printed cost, then the greedy credit when credit is set,
// then the cost statics. The offer's cast is plain (cast.go's impPlan arm: a
// hand cast, no X, no may-play, flashback or alternate shape), so nothing else
// folds here; convokeCreditFrom reads the offer's hand origin whatever zone the
// build has moved the card to.
func (s *State) improviseCreditPrice(k *prep, e *stackEntry, credit bool) kop.ManaCost {
	cost := k.cards[e.card].cost
	if credit {
		cost = s.convokeCreditFrom(k, SeatIx(e.ctrl), e.card, e.obj, true, cost)
	}
	if s.statM.any() || k.gen != nil {
		cost = s.featCost(k, SeatIx(e.ctrl), e.card, e.obj, cost)
	}
	return cost
}

// improvisePlanPay pays an Improvise cast gorge's planner priced, with the
// artifacts announced. gorge runs the plan armed at the offer (priced with the
// greedy credit's artifacts held back) while it still describes the payment: the
// same generic is left to pay and no step taps an announced artifact. Otherwise
// the plan is re-armed on what the announced artifacts leave (rearmAutoPay). The
// planner reads the seat's control mask, so the held-back artifacts leave that.
// It reports stranded (the cast was reversed) and paid (the plan tapped). A
// plan with no payment on what the artifacts leave is stranded too when no
// way through the window pays (gorge's paymentStranded: the dry strand check
// cannot price a sacrifice-only source, an Eldrazi Spawn, but the plan does,
// CR 733.1); when the window can (a restricted mana the spell admits, which
// the plan never counts, CR 106.6) it is neither, and the manual window pays.
func (s *State) improvisePlanPay(k *prep, e *stackEntry, cost *kop.ManaCost, anyColor bool, saved *[len(build{}.picks)]uint8) (stranded, paid bool) {
	occ := s.occ
	hidden := s.convokeHidden()
	printed := *cost
	*cost = s.convokeApplyPicks(k, xFolded(*cost, int(e.x)))
	s.occ = occ.andNot(hidden)
	if s.bld().bfl&bflMsDone == 0 && s.paymentStranded(k, e, cost) {
		s.occ = occ
		s.releaseCostTaps(k, saved)
		s.reverseCast(k)
		return true, false
	}
	s.occ = occ
	si := SeatIx(e.ctrl)
	ctrl := s.ctrlM[si]
	hold := hidden
	skipPlan := false
	if g := s.improviseReserved(k, e.ctrl, k.cards[e.card].cost.Generic); g.count() == hidden.count() || (!DebugPlannerFlexCensus && hidden.count() == 0) {
		c := *cost
		t := s.fork()
		t.ctrlM[si] = ctrl.andNot(g)
		conflict := !t.tapForTry(k, e.ctrl, &c, int(e.x), e.obj, anyColor)
		for m := hidden; m.any(); m = m.rest() {
			if t.perms[m.first()].flags&pfTapped != 0 {
				conflict = true
			}
		}
		s.unfork()
		if conflict && hidden.count() == 0 && !DebugPlannerFlexCensus {
			// The plan armed at the offer (priced with the greedy credit's
			// artifacts held back) replans to nothing, so the payment falls
			// back the way gorge's cost_changed does -- the manual CR 601.2g
			// window (manaStep).
			skipPlan = true
		} else if !conflict {
			hold = g
		}
	}
	if !skipPlan {
		s.ctrlM[si] = ctrl.andNot(hold)
		paid = s.tapForTry(k, e.ctrl, cost, int(e.x), e.obj, anyColor)
		// Restore the artifacts this payment withheld, not the whole saved mask: the
		// plan behind tapForTry may have sacrificed an Eldrazi Scion (or another
		// sacrifice source) to fund the cost, and removePerm cleared its control bit
		// while its occupancy bit went with it. Putting the old mask back would leave
		// the sacrificed source in ctrlM with no occupancy, and attackerOptions reads
		// ctrlM without s.occ: a stale token would be offered as an attacker.
		s.ctrlM[si] = s.ctrlM[si].or(hold.and(s.occ))
	}
	if paid {
		return false, true
	}
	s.occ = occ.andNot(hidden)
	strand := s.bld().bfl&bflMsDone == 0 && s.paymentStrandedPlan(k, e, cost, true)
	s.occ = occ
	if !strand {
		*cost = printed // the manual window applies the picks itself (manaStep)
		return false, false
	}
	s.releaseCostTaps(k, saved)
	s.reverseCast(k)
	return true, false
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
		// still describes the payment; stranded, the cast is reversed; with
		// no plan and a source the census does not price, the manual window pays
		stranded, paid := s.improvisePlanPay(k, e, cost, anyColor, saved)
		return paid || stranded, stranded
	}
	return false, false
}

// plainCastZone reports whether a cast from zone z is a plain cast to gorge's
// planner (scope ""): from a hand or, a commander, the command zone (CR 903.8).
func plainCastZone(z kop.Zone) bool { return z == kop.ZoneHand || z == kop.ZoneCommand }
