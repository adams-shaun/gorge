package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A printed trigger's own "you may pay {X}" (Shanna, Purifying
// Blade). Gorge's triggered-cost window (rules cumulative.go
// triggeredCostXAsk, the SVar:X = Count$xPaid payer-chooses arm) announces
// X BEFORE the pay / don't-pay election and the mana window: a KChoose of
// "trigger_cost_x" over 0..bound ascending, bound = the most mana the payer
// can produce (capped at 64, xFoldBoundCap) then the body's XMax$ (Shanna:
// the life gained this turn), keeping the X values the cost {X} is payable
// for. The answer folds into the cost: the ordinary ChGenPay election
// follows (pay is offered with a free cost at X = 0), then the usual step
// by step mana payment, and on payment the body reads Count$xPaid as the
// announced X (rp.winPaidX). kop.PayManaX is that cost: A the XMax$ cap, or
// 64 when the body names none (the mana bound alone). The bound is gorge's
// payment-aware one (rules cumulative.go triggeredCostXAsk).
// Cold: reached only from a PayManaX instruction.

// raTrigX is the announcement ask; ChTrigX its decision sub-kind (gorge's
// "trigger_cost_x" option, canonical like "x": the mode is the value).
const raTrigX uint8 = 249

// ChTrigX is the trigger-own X announcement.
const ChTrigX ChoiceKind = 249

// trigXCap is gorge's xFoldBoundCap.
const trigXCap = 64

// execTrigX poses the announcement; the payer's affordable X values are
// 0..the largest affordable one at most cap. False = suspended.
//
// The bound is gorge's payment-aware one (cumulative.go triggeredCostXAsk,
// potentialManaFor with the trigger cost as the payment): a Chosen-colour
// source whose colour is fixed makes exactly its chosen colour, where the
// plain over-bound prices the production open in every colour and offered X
// past what the sources can fund (CR 106.6; a cycled Decree of Justice's
// "you may pay {X}" beside a Sea Gate).
//
//go:noinline
func (s *State) execTrigX(k *prep, p uint8, skip uint16, cap int) bool {
	var av avail
	// a payment's potential, as gorge's potentialManaFor(desc): a Chosen-colour
	// source makes its chosen colour (a gate land's "{U} or the chosen colour"),
	// not an open source (CR 106.1a, 601.2b). A trigger's own cost is no spell's,
	// an ability's or a cast's, and has no source slot.
	other := payDesc{class: pdOther, src: NoSlot}
	open := s.availPotentialFor(k, p, &av, &other, false)
	cap = min(cap, trigXCap)
	mx := 0
	for x := 1; x <= cap; x++ {
		c := manaOf(x, 0)
		// a restricted source whose restriction admits a non-spell payment (the
		// Powerstone's {C}, CR 106.6) funds the trigger's cost too: gorge's
		// potentialManaFor prices it in the bound
		if !av.affords(&c, 0, 0) && !(k.restrSrc && s.restrSrcAffords(k, p, other, &c, &av, NoSlot, 0)) {
			break
		}
		mx = x
	}
	// The potential walk folds every face of a multi-ability source (Quirion
	// Elves' {G} and chosen-colour taps) so no colour is lost, which
	// overstates what one tap can pay. When the bound sits below the walk's
	// cap, cap it by availForCapped's per-source census (each source once, at
	// its largest production, including a source the planner defers or a
	// ColorIdentity source its units omit), as gorge's triggeredCostXSupply
	// caps the payment-aware bound. At the walk's cap the correction still
	// applies when the cap is a real one (an XMax$ the payer's life or the
	// body names): a walk that saturated it may have folded one source's
	// faces into two values (Volcanic Island's {U} and {R} at an XMax$ cap of
	// 2), and the census reads the board once per source, so the old skip
	// offered X past what the sources fund. Only the defensive cap (64)
	// skips it there: the walk may still be growing and the shapes whose
	// census is narrower than the walk -- a storage land's counters, an open
	// or granted source -- report open themselves, so a non-open walk at 64
	// is one the census could drop a really-payable value from.
	if !open && (mx < cap || cap < trigXCap) {
		// gorge's potentialManaFor prices a tap's extra mana (Mana Flare's) in
		// the bound, so the walk's count grows by it before the census below
		// caps it (CR 605.1b)
		mx = min(cap, mx+s.trigXTapExtras(k, p))
	}
	if !open && (mx < cap || cap < trigXCap) {
		var one avail
		one.xd, one.xdOn = other, true // a restricted grant the trigger's own payment may not spend adds no unit (CR 106.6)
		s.availForCapped(k, p, &one)
		one.xdOn = false
		if k.restrSrc {
			if t, _, nm := s.restrSrcAvailD(k, p, other, &one, NoSlot, false); nm == 0 {
				one = t // the restricted sources a non-spell payment may spend count once each too
			}
		}
		mx = min(mx, one.total+s.trigXLethalPain(k, p)-s.trigXFiltExcess(k, p))
	}
	s.g.pickF = skip
	s.g.pickZ = kop.PayManaX
	s.g.pickLeft = uint8(mx)
	s.g.pickP = p
	s.res.ask, s.res.askP = raTrigX, p
	return false
}

// trigXFiltExcess is the mana the census's gap units add for p's filters past
// their free taps (a Mystic Gate's "{W/U},{T}: Add two mana" beside its
// "{T}: Add {C}"). gorge's triggeredCostXSupply prices a source once, at the
// largest production among its fee-free alternatives (triggeredCostXFeeFree),
// so a filter with a free tap adds nothing: it turns one mana into two, and the
// tap gives one anyway (CR 605.3b). A filter that is its card's first and only
// ability has no free tap in gorge's census and is priced net of its fee by
// triggeredCostXPaidNet, so it keeps its units here.
func (s *State) trigXFiltExcess(k *prep, p uint8) int {
	if k.gen == nil || !k.gen.anyFilt {
		return 0
	}
	// the first-and-only test is per card: anyFiltFirst is only the table's
	// "some card has one", and a Mystic Gate beside such a card still has its
	// free tap
	return s.filtBoundExtraSkip(k, p, true)
}

// trigXTapExtras is the mana the controller's tap triggers add across every
// source that can tap for mana, each counted once (mana_tapextra.go srcExtra).
func (s *State) trigXTapExtras(k *prep, p uint8) (n int) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return 0
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		pc := s.manaPCGen(k, sl)
		if !s.canTapForManaPC(k, sl, pc) || pc.manaAb < 0 {
			continue
		}
		if pc.painCol != 0 {
			// a painful source is priced once at its larger alternative
			// (srcExtraPain: the normal ability's extra or the painful pick's)
			xc, xp, r, ok := s.srcExtraPain(k, sl, pc)
			if !ok {
				continue
			}
			nc, np := 0, int(r)
			for c := range xc {
				nc += int(xc[c])
				np += int(xp[c])
			}
			n += max(nc, np)
			continue
		}
		if s.srcEachPick(k, sl, pc) > 0 {
			// the pick floats unasked (nothing poses its colour ask yet);
			// srcExtra reads the production alone, so the bound would be one
			// short and the ask options smaller than gorge's
			s.capFull(StallTapExtra)
			continue
		}
		for _, v := range s.srcExtra(k, sl, pc) {
			n += int(v)
		}
	}
	return n
}

// askOptionsTrigX lists X = 0..pickLeft (the tail of askOptionsHide).
//
//go:noinline
func (s *State) askOptionsTrigX(k *prep, d *Decision) bool {
	if s.res.ask != raTrigX {
		return s.askOptionsManaChoice(k, d) // mana_colourchoice.go
	}
	d.Sub = ChTrigX
	for x := 0; x <= int(s.g.pickLeft); x++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(x)))
	}
	return true
}

// applyAskTrigX folds the announced X into the cost and poses the ordinary
// pay / don't-pay election on it (the tail of applyAskHide).
//
//go:noinline
func (s *State) applyAskTrigX(k *prep, a Action) bool {
	if s.res.ask != raTrigX {
		return s.applyAskManaChoice(k, a) // mana_colourchoice.go
	}
	if s.g.pickZ == payKindEvidence {
		return s.applyAskEvidenceX(k, a) // CollectEvidence<X> (cost_collectevidencex.go)
	}
	s.g.pickZ = kop.PayMana
	s.g.pickLeft = a.Mode()
	p := s.g.pickP
	ok := s.trigXCensusPays(k, p, int(a.Mode()), 0)
	var stall uint8
	if !ok {
		ok, stall = s.windowPays(k, p, kop.PayMana, s.g.pickF, int(a.Mode()), 0)
	}
	if !ok {
		ok = s.trigCostPotentialPays(k, p, kop.PayMana, s.g.pickF, int(a.Mode()), 0, 0)
	}
	if !ok {
		s.g.pickF |= pfNoPay // the X bound read no restriction; a pay reads the payment's (decline only)
	}
	if stall != 0 {
		// the drop is final (decline only) and a failed window probe's fork
		// latched a rule gap the kernel stalls by name: declare it, not a
		// silent unpayable X (tapXScript's shape). A found plan's spill is the
		// same model gap and is declared too.
		s.capFull(stall)
	}
	s.res.ask, s.res.askP = raGenPay, p
	return true
}

// trigXCensusPays is the trigger's own X election's affordability: the same
// one-tap census the announcement's bound caps by (execTrigX's
// availForCapped), so every announced X is payable (CR 601.2b: the announced
// X must be one the payment can settle). canPayGen's plain window census
// leaves a planner-deferred source out (the script gap the bound's census and
// the payment's window both count): a Condition-split source beside Mana
// Flare was offered X = 13 and answered decline only.
func (s *State) trigXCensusPays(k *prep, p uint8, amt int, pips int16) bool {
	c := s.numTimesCost(k, amt, pips)
	s.payManaHyb(k, 0, &c) // a trigger's own hybrid cost (cost_trighybrid.go)
	s.announcedHyb(&c)
	var av avail
	av.xd, av.xdOn = payDesc{class: pdOther}, true // an X-folded trigger cost is no spell's, an ability's or a cast's
	s.availForCapped(k, p, &av)
	if m := s.restrFloatAdmits(k, p, &av.xd); m != 0 {
		// the floating restricted mana this payment admits is pool too
		// (CR 106.6: gorge's unless, ward and trigger-cost gates price
		// AvailableFor's PurposeOther view, which restores every batch a
		// non-spell payment may spend)
		var rn [kop.NumManaKinds]uint8
		s.rbUnits(p, m, &rn)
		for ci, n := range rn {
			if n != 0 {
				av.addUnits(1<<uint(ci), n)
			}
		}
	}
	av.xdOn = false
	if av.affords(&c, pipMaskOf(&c), 0) {
		return true
	}
	// a restricted source whose spend restriction admits a non-spell payment
	// funds the trigger's cost too, as canPayGen's arm reads it (CR 106.6)
	return k.restrSrc && s.restrSrcAffords(k, p, payDesc{class: pdOther, src: NoSlot}, &c, &av, NoSlot, 0)
}

// trigXBind makes the paid X the resolving trigger's Count$xPaid.
//
//go:noinline
func (s *State) trigXBind(k *prep) {
	if in := &s.prog(k)[s.res.pc]; in.C&0xF == kop.PayManaX && in.B.N != kop.PayBSubEvidence {
		s.res.e.x = objWord(s.g.pickLeft)
	}
}
