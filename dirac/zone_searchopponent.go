package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Which opponent answers a library search an opponent answers (OpSearchEx
// under kop.SrChooserOpp: Realms Uncharted, Guided Passage), or a hidden pick
// outside the hand (OpChooseMove under SrChooserOpp: Tasigur, the Golden
// Fang's graveyard return; gorge bindHiddenPickOpponent). With two or more
// live opponents gorge asks the resolving controller which one before anything
// of the search is posed (beginSearchOwner; CR 608.2d), and the kernel asks it
// too: raGenSearchOpp (manifest_gen.go, dirac/manifest/ask), pileOppOptions'
// ChGenPlayer pick. The seat is kept for chooserOf until the next such search.

// useSearchOpp arms the picked-opponent byte for a table holding such a search.
var useSearchOpp = newGenUse(tableAnyOppSearch)

// coldSearchOpp is the picked opponent, seat + 1 (0 none: one opponent).
var coldSearchOpp = newCold[uint8]("searchOpp", func(g *genPrep) int {
	if g.uses.has(useSearchOpp) {
		return 1
	}
	return 0
})

// tableAnyOppSearch reports a library search an opponent answers.
func tableAnyOppSearch(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			prog := t.Cards[i].Abilities[j].Prog
			for n := range prog {
				if oppSearch(t, &prog[n]) {
					return true
				}
			}
		}
	}
	return false
}

// oppSearch reports an opponent-answered search or hidden pick. Over a hand
// pool a ChooseMove's B bit 0 is the UntilLeaves hand-exile link instead
// (chooserOf), so a hand pick is never one.
func oppSearch(t *kop.Table, in *kop.Instr) bool {
	if in.B.N&kop.SrChooserOpp == 0 {
		return false
	}
	switch in.Op {
	case kop.OpSearchEx:
		return true
	case kop.OpChooseMove:
		return int(in.F) < len(t.Filters) && t.Filters[in.F].Zone != kop.ZoneHand
	}
	return false
}

// DebugSearchOppNext is the lockstep mutation of the opponent pick: the next
// live seat answers the search whatever the controller picked, so
// TestFourSeatChooserOpponentLockstep and
// TestFourSeatHiddenPickChooserOpponentLockstep must diverge. Never set in production.
var DebugSearchOppNext bool

// searchOppAsk forgets the last search's opponent and, for an opponent-answered
// search with two or more live opponents, poses raGenSearchOpp. true: asked.
func (s *State) searchOppAsk(k *prep, in *kop.Instr) bool {
	if !oppSearch(k.t, in) || in.Op == kop.OpChooseMove && s.searchZone(k) == kop.ZoneHand {
		return false
	}
	c := coldSearchOpp.of(s)
	if c == nil {
		return false
	}
	c[0] = 0
	ctrl := s.res.e.ctrl
	if s.oppsFrom(ctrl).n < 2 {
		return false
	}
	s.res.ask, s.res.askP = raGenSearchOpp, ctrl
	return true
}

// searchOppAnswer keeps the picked opponent and opens the search (the same op,
// its pc unmoved).
func (s *State) searchOppAnswer(k *prep, a Action) {
	coldSearchOpp.of(s)[0] = a.Mode() + 1
	s.res.ask = raNone
	if s.openSearch(k, &s.prog(k)[s.res.pc]) {
		s.continueRes(k, 0)
	}
}

// searchOpponent is the opponent who answers the pick of in: the one the
// controller picked for an opponent-answered library search, else the
// controller's first live opponent. 255: none.
func (s *State) searchOpponent(k *prep, in *kop.Instr) uint8 {
	if oppSearch(k.t, in) && !DebugSearchOppNext {
		if c := coldSearchOpp.of(s); c != nil && c[0] != 0 {
			return c[0] - 1
		}
	}
	return s.playerRef(k, kop.RefOpponent)
}
