package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ext Clash (kop.ExtClash; CR 701.31, gorge effClash in effects/clash.go):
// the resolving controller clashes with their opponent. Both reveal their top
// card; the controller's and then the opponent's own card is put on the top or
// the bottom of its owner's library (one ChGenClash ask each, in that order:
// APNAP, CR 101.4; a player with an empty library has no card and is not
// asked); and the player whose card has the strictly higher mana value wins (a
// tie, two empty libraries, or the controller's empty library, leaves no
// winner, as gorge reads an empty library as -1).
//
// The verdict is struck when the cards are revealed, before either card moves
// (gorge reads both mana values first), and is left in the resolution's result
// slot (g.roll, ExRollResult, the slot FlipCoin's branches read): 1 the
// controller won, 2 anything else. WinSubAbility$ / OtherwiseSubAbility$ lower
// as blocks guarded by it (kgen/lower_clash.go). The ask needs no scratch: the
// revealed card is still the top of the asked player's own library when it is
// answered, and the controller's ask is the only one whose answer is followed
// by another.
//
// The options are gorge's, bottom (mode 0) then top (mode 1), over the
// revealed card.

// The clash opponent (CR 701.31a; gorge clashParticipants ->
// chooseClashOpponent): the controller chooses one living opponent, an ask
// (raPodClashOpp, ChGenPlayer, mode = the seat) of one "player" option per
// opponent in AliveFrom(0) order, posed only with two or more of them; a sole
// opponent is the clash's unasked and none leaves the controller clashing
// alone. The seat is held in res.chPl (seat + 1) until the last placement is
// answered, then cleared, so no ChosenPlayer reader sees it (kgen refuses a
// Clash beside a ChoosePlayer of its own ability).

// clashOpp is the seat the controller clashes with (res.chPl, set as the
// clash starts), or noSeat when none is left.
func (s *State) clashOpp(p uint8) uint8 {
	if s.res.chPl != 0 {
		return s.res.chPl - 1
	}
	return noSeat
}

// DebugClashNextSeat is the lockstep mutation of the pod clash: the kernel
// clashes with the next live seat unasked, the two-seat reading
// (TestFourSeatClashLockstep must then diverge).
var DebugClashNextSeat bool

// clashMV is the mana value of the top card of seat p's library, -1 for an
// empty library.
func (s *State) clashMV(k *prep, p uint8) int {
	pl := &s.pl[p]
	if pl.libLen() == 0 {
		return -1
	}
	o := pl.lib[pl.libLo]
	return s.objMV(k, s.objs[o].card, o)
}

// clashExt is OpExt ExtClash; false: the opponent or a placement ask is
// posed.
//
//kmanifest:ext ExtClash
func (s *State) clashExt(k *prep, in *kop.Instr) (ok, done bool) {
	p := s.res.e.ctrl
	q := s.oppsFrom(p)
	if q.n > 1 && !DebugClashNextSeat {
		s.res.ask, s.res.askP = raPodClashOpp, p
		return false, true
	}
	opp := uint8(noSeat)
	if q.n > 0 {
		opp = q.s[0]
	}
	return s.clashStart(k, p, opp), true
}

// clashStart reveals and strikes the verdict of p's clash with opp (noSeat:
// none) and poses the first placement ask; true: nothing is left to ask.
func (s *State) clashStart(k *prep, p, opp uint8) bool {
	s.res.chPl = 0
	mine, theirs := s.clashMV(k, p), -1
	if opp != noSeat {
		s.res.chPl = opp + 1
		theirs = s.clashMV(k, opp)
	}
	s.g.roll = 2
	if mine >= 0 && mine > theirs {
		s.g.roll = 1
	}
	if s.clashAsk(p) {
		s.res.chPl = 0
		return true
	}
	return false
}

// clashOppOptions fills the opponent ask: one option per living opponent of
// the controller in seat order from seat 0 (gorge's AliveFrom(0)).
func (s *State) clashOppOptions(d *Decision) {
	d.Sub = ChGenPlayer
	for q := uint8(0); q < s.nSeats(); q++ {
		if q != s.res.askP && !s.out(q) {
			d.add(MakeAction(AkChoose, 0, 0, q))
		}
	}
}

// clashOppAnswer starts the clash with the chosen opponent.
func (s *State) clashOppAnswer(k *prep, a Action) {
	s.res.ask = raNone
	if s.clashStart(k, s.res.e.ctrl, a.Mode()) {
		s.continueRes(k, 0)
	}
}

// clashAsk poses the placement ask of seat p, or of the opponent when p's
// library is empty and p is the controller; true: nothing is left to ask.
func (s *State) clashAsk(p uint8) bool {
	if s.pl[p].libLen() > 0 {
		s.res.ask, s.res.askP = raGenClash, p
		return false
	}
	if p == s.res.e.ctrl {
		if o := s.clashOpp(p); o != noSeat {
			return s.clashAsk(o)
		}
	}
	return true
}

// clashOptions poses gorge's options, bottom then top.
func (s *State) clashOptions(d *Decision) {
	d.Sub = ChGenClash
	pl := &s.pl[s.res.askP]
	top := pl.lib[pl.libLo]
	d.add(MakeAction(AkChoose, TCode(top), 0, 0))
	d.add(MakeAction(AkChoose, TCode(top), 0, 1))
}

// clashAnswer places the asked player's card, then asks the opponent when the
// controller answered.
func (s *State) clashAnswer(k *prep, a Action) {
	r := &s.res
	p := r.askP
	r.ask = raNone
	pl := &s.pl[p]
	if a.Mode() == 0 && pl.libLen() > 1 {
		o := pl.lib[pl.libLo]
		pl.libLo++
		s.touch(o)
		pl.libBottom(o)
	}
	next := true
	if p == r.e.ctrl {
		if o := s.clashOpp(p); o != noSeat {
			next = s.clashAsk(o)
		}
	}
	if next {
		s.res.chPl = 0
		s.continueRes(k, 0)
	}
}
