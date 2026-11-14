package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// OpTwoPiles' players and its unchosen pile (gorge effTwoPiles' DefinedCards$
// Remembered arm, CR 700.3). The split rides the remembered set: pile A is the
// separator's picks (armRes.pileOrd, in pick order, under lookSel), pile B the
// rest in the revealed order.

// pilesSeats is the separator and the chooser, as gorge's twoPilesPlayer
// reads Separator$ and Chooser$ at TWO seats: the resolving controller and
// the first live opponent, swapped under kop.TpOppSeparates (Fact or
// Fiction). Every TwoPiles carrier lowers two-seat: at three or more seats
// gorge asks the spell's controller WHICH opponent separates or picks (CR
// 801.5b, twoPilesAskOpponent), a decision dirac does not model, so the
// kernel stalls StallTwoSeat on the carrier instead of playing it.
func (s *State) pilesSeats(in *kop.Instr, ctrl uint8) (sep, chooser uint8) {
	opp := s.nextAlive(ctrl)
	if in.B.N&kop.TpOppSeparates != 0 {
		return opp, ctrl
	}
	return ctrl, opp
}

// pilesTooWide reports a remembered set wider than pileOrd, the split's pick
// order (Epiphany at the Drownyard reveals X plus one): a declared capacity
// stop, as a dig window wider than look is.
func (s *State) pilesTooWide() bool {
	if int(s.g.nrem) <= len(s.resArm().pileOrd) {
		return false
	}
	s.capFull(StallLook)
	return true
}

// pilesToGraveyard sends the pile cards still in the library to the graveyard
// in gorge's order: an unchosen pile A in pick order under
// kop.TpUnchosenPileOrder (Fact or Fiction, Steam Augury), then the rest in
// the revealed order (without the flag the revealed order is all, the library
// order Curator's ChangeZoneAll sweep reads). Gorge moves them one at a time,
// so from the first card whose move two exile-instead replacements compete
// for (Rest in Peace twice, CR 616.1) that card and every later one park for
// the order ask (parkPile); the resolution goes on after the last answer.
func (s *State) pilesToGraveyard(k *prep, in *kop.Instr, np int, pickA bool) {
	var ord [12]Obj // pileOrd's width bounds the split (pilesTooWide)
	n := 0
	add := func(o Obj) {
		if s.objs[o].zone != kop.ZoneLibrary {
			return
		}
		for _, p := range ord[:n] {
			if p == o {
				return
			}
		}
		ord[n] = o
		n++
	}
	if !pickA && in.B.N&kop.TpUnchosenPileOrder != 0 {
		for j := 0; j < np; j++ {
			add(s.g.rem[s.resArm().pileOrd[j]])
		}
	}
	for i := 0; i < int(s.g.nrem); i++ {
		add(s.g.rem[i])
	}
	first := n
	if k.gen != nil {
		for i, o := range ord[:n] {
			if s.graveMoveCompetes(k, o) {
				first = i
				break
			}
		}
	}
	for _, o := range ord[:first] {
		s.pileMove(o, kop.ZoneGraveyard)
	}
	for _, o := range ord[first:n] {
		if !s.park(k, o, parkPile, 0) && !s.parkQuiet(o, parkPile, 0) {
			s.capFull(StallReplCands)
			return
		}
	}
}
