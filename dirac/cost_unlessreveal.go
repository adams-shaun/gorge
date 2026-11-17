package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// UnlessCost$ Reveal<N/Spec> as a resolution-time cost (Priest of the
// Wakening Sun's "you may reveal a Dinosaur card from your hand. If you do,
// you gain 2 life"): gorge poses the pay / decline election (pay only while
// pay.unlessComponentsPayable holds), then beginUnlessPayment's walk asks the
// "revealcost" pick over the payer's matching hand cards — unasked with
// exactly the count of candidates, asked over more — and settles the picks as
// one public Note on the paid path, nothing moved (pay/unlessflow.go
// AdvanceUnless). It rides PayDiscard with B.N kop.PayBReveal (C's nibble is
// full); unlessRevealPayKind widens it to payKindReveal.

// payKindReveal is the pay kind (execPay's kind, pickZ) of an UnlessCost$
// Reveal<N/Spec>.
const payKindReveal = 25

// unlessRevealPayKind widens a PayDiscard whose B.N is PayBReveal to
// payKindReveal.
func unlessRevealPayKind(kind int, bn int16) int {
	if kind == kop.PayDiscard && bn == kop.PayBReveal {
		return payKindReveal
	}
	return kind
}

// unlessRevealCands counts player p's hand candidates for the reveal filter f,
// adding each to d when d is set: the matching hand cards in hand order (gorge
// unlessflow's "revealcost" candidates, pay.UnlessCandidatesFor over the hand).
//
//go:noinline
func (s *State) unlessRevealCands(k *prep, p uint8, f kop.FilterID, d *Decision) int {
	n := 0
	pl := &s.pl[SeatIx(p)]
	for i := 0; i < int(pl.handN); i++ {
		if o := pl.hand[i]; s.matchCardGen(k, f, s.objs[o].card, SeatIx(p), SeatIx(p), o) {
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			}
		}
	}
	return n
}

// unlessRevealOptions poses the pending pick: an UnlessCost$ Reveal<N/Spec>'s
// pick (ChCostReveal, gorge's "revealcost" kind: source = the card revealed,
// repl_revealland.go) or, for a Behold cost, the behold pick it dispatches to
// (cost_unlessbehold.go).
//
//go:noinline
func (s *State) unlessRevealOptions(k *prep, d *Decision, p uint8) {
	if s.g.pickZ == payKindBehold {
		s.unlessBeholdOptions(k, d, p)
		return
	}
	d.Sub = ChCostReveal
	s.unlessRevealCands(k, p, s.genPickFilter(k), d)
}

// unlessRevealPick reports that the pending pay pick is an UnlessCost$
// Reveal<N/Spec>, or the Behold cost that shares its option and answer
// dispatch.
func (s *State) unlessRevealPick() bool {
	z := s.g.pickZ
	return z == payKindBehold || z == payKindReveal
}

// unlessRevealAnswer applies one answered reveal pick and finishes the
// payment: the card stays in hand and gorge announces the settled picks as one
// Note on the paid path (pay/unlessflow.go), which moves no state. A Behold
// pick is always one and finishes at once. It reports whether the payment
// finished (the caller breaks) or another pick is owed (it returns).
//
//go:noinline
func (s *State) unlessRevealAnswer(k *prep) bool {
	if s.g.pickZ == payKindBehold {
		s.g.pickLeft = 0
		s.payDone(k, true)
		return true
	}
	s.g.pickLeft--
	if s.g.pickLeft > 0 {
		return false
	}
	s.payDone(k, true)
	return true
}
