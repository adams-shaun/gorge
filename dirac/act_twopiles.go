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

// pilesUnchosenPicks moves an unchosen pile A to the graveyard in pick order
// under kop.TpUnchosenPileOrder: gorge's ChangeZone Defined$ Remembered body
// walks the pile as answered (Fact or Fiction, Steam Augury). Without the flag
// the caller's walk moves it in the revealed order, the library order
// Curator's ChangeZoneAll sweep reads.
func (s *State) pilesUnchosenPicks(in *kop.Instr, np int) {
	if in.B.N&kop.TpUnchosenPileOrder == 0 {
		return
	}
	for j := 0; j < np; j++ {
		s.pileMove(s.g.rem[s.resArm().pileOrd[j]], kop.ZoneGraveyard)
	}
}
