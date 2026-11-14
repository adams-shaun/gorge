package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// choosePlayerOp is OpChoosePlayer (gorge effChoosePlayer's Random$ True
// branch over Choices$ Player.Opponent+!EnchantedBy, Maddening Hex's
// re-attach): the pool is the chooser's living opponents (gorge
// MatchesPlayerSpecWithSVars' Opponent base read against the chooser,
// AliveFrom(chooser) order) that the source Aura is not attached to
// (!EnchantedBy is player.isEnchantedBy(source): a source off the battlefield
// fails closed and excludes nobody). The engine rng picks ONE of them (gorge
// randomChoices: rng.IntN(len(pool)) over the pool in that order) and the
// pick is the resolution's chosen player (res.chPl, seat + 1, 0 none; RefResChosenPlayer).
// An empty pool draws nothing (gorge randomChoices clamps the pick count to
// the pool) and records no choice: a following ChosenPlayer read — Maddening
// Hex's Attach — resolves to none, and gorge's effAttach EnchantSelf walk
// over the empty Defined$ set attaches nothing. gorge poses no decision
// either way (the Random$ branch records without an ask) and emits the
// choice's Note-level events only, which change no rules state.
//
//go:noinline
func (s *State) choosePlayerOp(k *prep, in *kop.Instr) {
	p := s.playerRef(k, in.Obj)
	if p == 255 || p >= s.nSeats() || s.out(p) {
		// no chooser (gorge choiceChoosers with an unresolvable Defined$): no
		// pick is recorded
		s.res.chPl = 0
		return
	}
	src := s.objRef(k, kop.RefSelf)
	ench := uint8(0)
	if src != NoSlot && s.objs[s.perms[src].obj].zone == kop.ZoneBattlefield {
		ench = s.atchSeat(k, src)
	}
	var pool [MaxSeats]uint8
	n := 0
	q := s.oppsFrom(p)
	for i := 0; i < int(q.n); i++ {
		if ench != 0 && q.s[i]+1 == ench {
			continue
		}
		pool[n] = q.s[i]
		n++
	}
	if n == 0 || in.C&kop.ChPlRandom == 0 {
		s.res.chPl = 0
		return
	}
	s.res.chPl = pool[s.rng.IntN(n)] + 1
}
