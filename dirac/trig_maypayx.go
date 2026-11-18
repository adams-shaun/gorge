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
	// overstates what one tap can pay. When the bound sits below the
	// walk's cap (the defensive cap, or a lower XMax$), cap it by
	// availForCapped's per-source census (each source once, at its largest
	// production, including a source the planner defers or a ColorIdentity
	// source its units omit), as gorge's triggeredCostXSupply caps the
	// payment-aware bound. At the walk's cap the correction is skipped: the
	// census is narrower than the walk (a storage land's counters, an open or
	// granted source), so narrowing there could drop a value the payment can
	// really pay.
	if !open && mx < cap {
		var one avail
		s.availForCapped(k, p, &one)
		if k.restrSrc {
			if t, _, nm := s.restrSrcAvailD(k, p, other, &one, NoSlot, false); nm == 0 {
				one = t // the restricted sources a non-spell payment may spend count once each too
			}
		}
		mx = min(mx, one.total)
	}
	s.g.pickF = skip
	s.g.pickZ = kop.PayManaX
	s.g.pickLeft = uint8(mx)
	s.g.pickP = p
	s.res.ask, s.res.askP = raTrigX, p
	return false
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
	ok := s.canPayGen(k, p, kop.PayMana, int(a.Mode()), 0, 0)
	var stall uint8
	if !ok {
		ok, stall = s.windowPays(k, p, kop.PayMana, s.g.pickF, int(a.Mode()), 0)
	}
	if !ok {
		ok = s.trigCostPotentialPays(k, p, kop.PayMana, s.g.pickF, int(a.Mode()), 0, 0)
	}
	if !ok {
		s.g.pickF |= pfNoPay // the X bound read no restriction; a pay reads the payment's (decline only)
		if stall != 0 {
			// the drop is final (decline only) and a failed window probe's fork
			// latched a rule gap the kernel stalls by name: declare it, not a
			// silent unpayable X (tapXScript's shape)
			s.capFull(stall)
		}
	}
	s.res.ask, s.res.askP = raGenPay, p
	return true
}

// trigXBind makes the paid X the resolving trigger's Count$xPaid.
//
//go:noinline
func (s *State) trigXBind(k *prep) {
	if in := &s.prog(k)[s.res.pc]; in.C&0xF == kop.PayManaX && in.B.N != kop.PayBSubEvidence {
		s.res.e.x = objWord(s.g.pickLeft)
	}
}
