package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Reveal<1/Spec> as a cast-route cost (CR 701.20a, 601.2b; gorge cast_asks.go
// revealCostAsk and morph_turnup.go turnUpRevealAsk): the part one of a
// spell's either-or additional cost (AlternateAdditionalCost
// Reveal<1/Type>:<mana>, Daring Buccaneer) and the turn-up cost of a Morph
// family keyword (K:Morph:Reveal<1/Card.Blue>, Dragon's Eye Savants) reveal
// one card from the payer's hand matching CardDef.RevealF. Revealing moves
// and marks nothing, so an answer records no pick, and the cast card is never
// its own candidate (it is on the stack for a cast, on the battlefield for a
// turn-up). Exactly one candidate is taken unasked; more pose gorge's
// "revealcost" KChoose (ChCostReveal, source = the card revealed); fewer
// withhold the offer. A manifested or cloaked card's printed-mana-cost turn-up
// is no keyword turn-up and owes no reveal (CR 701.40b, 701.58b).
// Cold and out of line: table data and the build's pickK / donePicks.

// pkReveal marks the cast reveal pick (CostRemoveCtr's bit is no pick kind here). It
// carries pkExile's bit so the answer routes to answerCostObj, as pkBehold does.
const pkReveal = pickKind(kop.CostRemoveCtr) | pkExile

// revealPickCands counts (and, with d != nil, lists as AkChoose{source: the
// card}) the hand cards of p matching card c's RevealF in hand order, the
// card src excepted (gorge CostCandidates ZHand with excludeSource).
func (s *State) revealPickCands(k *prep, p uint8, c CardID, src Obj, d *Decision) int {
	f := k.t.Cards[c].RevealF
	pl := &s.pl[p]
	n := 0
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		if o == src || !s.matchCardGen(k, f, s.objs[o].card, p, p, o) {
			continue
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return n
}

// revealPayable: card c's either-or additional cost has no reveal part one,
// or p has a card to reveal for it (src: the cast card, no candidate).
func (s *State) revealPayable(k *prep, p uint8, c CardID, src Obj) bool {
	cd := &k.t.Cards[c]
	return cd.RevealN == 0 || !cd.HasAltAdd || s.revealPickCands(k, p, c, src, nil) >= int(cd.RevealN)
}

// revealCastOptions poses the reveal ask of the spell being built (bsCosts,
// ahead of the other cost-object picks, as beholdOptions): false when none is
// owed, the choice made, or the lone candidate taken unasked.
func (s *State) revealCastOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if e.kind != seSpell || s.bld().donePicks&pkReveal != 0 {
		return false
	}
	cd := &k.t.Cards[e.card]
	if cd.RevealN == 0 || !cd.HasAltAdd || s.bld().altAdd != 0 {
		return false
	}
	d.N = 0
	n := s.revealPickCands(k, e.ctrl, e.card, e.obj, d)
	if n < int(cd.RevealN) {
		// gorge offers every either-or part, the unpayable ones last, and a
		// reveal part chosen without a candidate aborts the cast here
		// (revealCostAsk; CR 601.2h, 733.1).
		d.N = 0
		s.reverseCast(k)
		return true
	}
	if n == int(cd.RevealN) {
		s.bld().donePicks |= pkReveal
		d.N = 0
		return false
	}
	s.bld().pickK, s.bld().pickLeft = pkReveal, cd.RevealN
	d.Kind, d.Sub, d.Player = DkChoose, ChCostReveal, e.ctrl
	return true
}

// answerCastReveal consumes the reveal answer: nothing moves or is marked.
func (s *State) answerCastReveal() {
	s.bld().donePicks |= pkReveal
	s.bld().pickK, s.bld().pickLeft = 0, 0
}

// turnUpRevealOwed: the keyword turn-up of the permanent at sl (mode 0, 3, 4
// or 5; kw) reveals a hand card.
func (s *State) turnUpRevealOwed(k *prep, sl Slot, kw bool) bool {
	cd := &k.t.Cards[s.cardAt(sl)]
	return kw && cd.RevealN != 0 && cd.MorphFamily != 0
}

// turnUpRevealOK: p has a card to reveal for the keyword turn-up of the
// permanent at sl (or it owes none).
func (s *State) turnUpRevealOK(k *prep, p uint8, sl Slot) bool {
	return !s.turnUpRevealOwed(k, sl, true) || s.revealPickCands(k, p, s.cardAt(sl), NoObj, nil) >= int(k.t.Cards[s.cardAt(sl)].RevealN)
}

// turnUpKwPayable: the keyword turn-up's non-mana cost, its life and its
// reveal, is payable by p.
func (s *State) turnUpKwPayable(k *prep, p uint8, sl Slot) bool {
	return s.turnUpLifeOK(k, p, sl) && s.turnUpRevealOK(k, p, sl)
}

// turnUpRevealStage runs the turn-up's reveal before it settles (gorge
// turnUpRevealAsk, ahead of the payment): exactly one candidate is taken
// unasked, several park the turn-up on ctlTurnUpReveal. It reports whether the
// turn-up is parked.
func (s *State) turnUpRevealStage(k *prep, o Obj, p uint8, mode uint8, kw bool) bool {
	sl := s.objs[o].slot
	if !s.turnUpRevealOwed(k, sl, kw) {
		return false
	}
	if s.revealPickCands(k, p, s.cardAt(sl), NoObj, nil) == int(k.t.Cards[s.cardAt(sl)].RevealN) {
		return false
	}
	s.g.pickP, s.g.pickZ, s.g.pickF = p, mode, uint16(o)
	s.ctl = ctlTurnUpReveal
	return true
}

// turnUpRevealOptions poses the reveal pick of a parked turn-up.
func (s *State) turnUpRevealOptions(k *prep, d *Decision) {
	d.N = 0
	d.Kind, d.Sub, d.Player = DkChoose, ChCostReveal, s.g.pickP
	s.revealPickCands(k, s.g.pickP, s.cardAt(s.objs[Obj(s.g.pickF)].slot), NoObj, d)
}

// applyTurnUpReveal takes the revealed card of a parked turn-up and settles it.
func (s *State) applyTurnUpReveal(k *prep) {
	p, mode, o := s.g.pickP, s.g.pickZ, Obj(s.g.pickF)
	s.ctl = ctlPriority
	s.settleTurnUp(k, o, p, mode, 0)
}
