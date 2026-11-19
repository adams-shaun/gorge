package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// OpTwoPiles' players and its unchosen pile (gorge effTwoPiles' DefinedCards$
// Remembered arm, CR 700.3). The split rides the remembered set: pile A is the
// separator's picks (armRes.pileOrd, in pick order, under lookSel), pile B the
// rest in the revealed order.

// pilesSeats is the separator and the chooser, as gorge's twoPilesPlayer
// reads Separator$ and Chooser$: the resolving controller and an opponent,
// swapped under kop.TpOppSeparates (Fact or Fiction). With one live opponent
// it is that opponent; with more, gorge asks the spell's controller WHICH
// opponent (CR 801.5b, twoPilesAskOpponent), the kernel asks it too
// (pilesOppAsk), and res.chPl carries the answer across the split: under
// TpOppSeparates that opponent separates, under kop.TpOppChooses (Curator of
// Destinies, Steam Augury) the controller separates and that opponent picks.
func (s *State) pilesSeats(in *kop.Instr, ctrl uint8) (sep, chooser uint8) {
	opp := s.nextAlive(ctrl)
	if s.res.chPl != 0 && !DebugPilesNextOpp {
		opp = s.res.chPl - 1
	}
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

// raGenPileOpp (manifest_gen.go, dirac/manifest/ask): OpTwoPiles' CR 801.5b
// ask under kop.TpOppSeparates or kop.TpOppChooses, which opponent separates
// or chooses (gorge twoPilesAskOpponent: KChoose of one "player" option per
// live opponent, AliveFrom the controller; ChGenPlayer, mode = the seat).

// DebugPilesNextOpp is the lockstep mutation of the opponent pick: the next
// live seat separates or chooses whatever the controller answered, the
// two-seat reading, so TestFourSeatTwoPilesLockstep and
// TestFourSeatTwoPilesChooserLockstep must diverge. Never set in production.
var DebugPilesNextOpp bool

// pilesOppAsk poses raGenPileOpp before the split when the controller has two
// or more live opponents and has not picked one yet; gorge asks it before the
// split (and with one card too), never with one opponent. true: asked.
func (s *State) pilesOppAsk(in *kop.Instr, ctrl uint8) bool {
	if in.B.N&(kop.TpOppSeparates|kop.TpOppChooses) == 0 || s.res.chPl != 0 || s.oppsFrom(ctrl).n < 2 {
		return false
	}
	s.res.ask, s.res.askP = raGenPileOpp, ctrl
	return true
}

// pileOppOptions fills raGenPileOpp: the controller's live opponents in
// AliveFrom order.
func (s *State) pileOppOptions(d *Decision) {
	d.Sub = ChGenPlayer
	q := s.oppsFrom(s.res.askP)
	for i := uint8(0); i < q.n; i++ {
		d.add(MakeAction(AkChoose, 0, 0, q.s[i]))
	}
}

// pileOppAnswer keeps the picked opponent and poses the split as OpTwoPiles
// would have (the same op, its pc unmoved).
func (s *State) pileOppAnswer(k *prep, a Action) {
	s.res.chPl = a.Mode() + 1
	in := &s.prog(k)[s.res.pc]
	sep, chooser := s.pilesSeats(in, s.res.e.ctrl)
	s.g.lookSel = 0
	s.res.ask, s.res.askP = raGenPileSplit, sep
	if s.g.nrem == 1 {
		s.g.lookSel, s.resArm().pileOrd[0] = 1, 0
		s.res.ask, s.res.askP = raGenPileChoose, chooser
	}
}
