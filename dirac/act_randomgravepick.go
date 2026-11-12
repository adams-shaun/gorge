package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// randomGravePick is OpChooseMove with SrRandom (Tersa Lightshatter's
// "exile a card at random from your graveyard", Ghoulraiser's and Make a
// Wish's returns to the hand, Moldgraf Monstrosity's to the battlefield): gorge effHiddenPick with
// AtRandom$ True takes the engine's seeded pick instead of asking
// (randomObjIDs): the eligible cards in graveyard order; a pool no larger
// than the count is taken whole without a draw, else Rand(len(rest)) per
// pick with the pick removed from the pool. The picks join s.resArm().look and the
// search finishes as an answered one would (moveFound moves them and, with
// SrRemember, remembers them).
//
//go:noinline
func (s *State) randomGravePick(k *prep) {
	if s.searchZone(k) == kop.ZoneExile {
		s.randomExilePick(k) // zone_exiledwithpick.go: the game-wide exile pool
		return
	}
	in := &s.prog(k)[s.res.pc]
	p := s.g.pickP
	pl := &s.pl[p]
	var rest [MaxDeck]Obj
	m := 0
	for _, o := range pl.grave[:pl.graveN] {
		if s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o) {
			rest[m] = o
			m++
		}
	}
	n := int(s.g.pickLeft)
	if n >= m {
		n = m
		for i := 0; i < m; i++ {
			s.resArm().look[i] = rest[i]
		}
	} else {
		for i := 0; i < n; i++ {
			j := s.rng.IntN(m)
			s.resArm().look[i] = rest[j]
			copy(rest[j:], rest[j+1:m])
			m--
		}
	}
	s.g.nlook = uint8(n)
	s.finishSearchNow(k)
}
