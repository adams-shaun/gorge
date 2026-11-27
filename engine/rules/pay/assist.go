package pay

import "github.com/adams-shaun/gorge/engine/state"

// assist.go is CR 702.132 Assist: another player paying part of a spell's
// generic mana cost while its controller pays the rest. It is the ONE home of
// the assist answer rule: what a seat may contribute is exactly what its own
// pool can pay of the spell's outstanding generic requirement, priced by the
// same pool view (AvailableFor) and the same resolver (ResolveManaWith, no
// conversion, no pip rider) PayMana spends with. The cast's ask builds its
// option list from AssistCap and the drain never exceeds AssistCap, so the
// offer, the answer Decision.Validate admits and the mana that actually leaves
// the seat's pool are one rule and cannot drift apart.

// AssistCap is the most of a spell's outstanding generic cost seat p may
// contribute right now: want, lowered until p's pool actually pays it. want is
// the CR 702.132a ceiling (the spell's generic part, or a printed
// "Assist<N>"), so the cap is the min of the two bounds and of the pool.
func AssistCap(e Engine, p state.PlayerID, want int32) int32 {
	for n := want; n > 0; n-- {
		if assistPayable(e, p, n) {
			return n
		}
	}
	return 0
}

// assistPayable prices n generic mana of an other-player contribution against
// p's payment-visible pool. The contribution carries no pips and no life part,
// so the closed-life re-resolution a pipped or life-paying cost needs cannot
// change its answer, and no payer grant (PayLifeInsteadOf:B, a may-play
// colour rider, a ManaConvert set) can widen a bare generic payment: it is the
// plain resolve PayMana will run on it.
func assistPayable(e Engine, p state.PlayerID, n int32) bool {
	if n <= 0 {
		return true
	}
	c := Cost{Generic: n}
	av := AvailableFor(e, p, Descriptor{Class: PurposeOther, Cost: &c})
	pl := e.Game().Players[p]
	_, ok := ResolveManaWith(c, av.Pool, pl.Snow, av.Typed, pl.Life, false, PipRider{}, nil)
	return ok
}

// AssistPay drains an assisting seat's contribution out of its OWN pool through
// the ordinary mana payment — the negative ManaAdd events PayMana emits are
// what make a foreign seat's spend replayable — and reports what it actually
// paid. That is at most want and at most AssistCap: a seat whose pool fell
// short of the amount it named contributes what it can, and the caster is
// charged the rest. A failed payment emits nothing (the payment core resolves
// before it spends), so a short pool never leaves a half-paid cost behind.
func AssistPay(e Engine, p state.PlayerID, want int32) int32 {
	n := AssistCap(e, p, want)
	if n == 0 || !PayMana(e, p, Cost{Generic: n}) {
		return 0
	}
	return n
}
