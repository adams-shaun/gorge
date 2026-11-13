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
// 64 when the body names none (the mana bound alone).
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
//go:noinline
func (s *State) execTrigX(k *prep, p uint8, skip uint16, cap int) bool {
	var av avail
	s.availPotential(k, p, &av)
	cap = min(cap, trigXCap)
	mx := 0
	for x := 1; x <= cap; x++ {
		c := manaOf(x, 0)
		if !av.affords(&c, 0, 0) {
			break
		}
		mx = x
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
	if !s.canPayGen(k, p, kop.PayMana, int(a.Mode()), 0, 0) && !s.windowPays(k, p, kop.PayMana, s.g.pickF, int(a.Mode()), 0) &&
		!s.trigCostPotentialPays(k, p, kop.PayMana, s.g.pickF, int(a.Mode()), 0, 0) {
		s.g.pickF |= pfNoPay // the X bound read no restriction; a pay reads the payment's (decline only)
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
