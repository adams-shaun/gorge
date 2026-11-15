package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a hidden ChangeZone pick whose origin is Exile
// (Omenpath Journey's AtRandom$ pick, Northampton Farm's GainControl$ pick,
// both over Card.ExiledWithSource). gorge effHiddenPick is game-wide here (no
// hand or library origin, no DefinedPlayer$): the pool is every player's exile,
// seat 0 first, in zone order (cardCands' precedent). Each seat's cards are
// matched with that seat as the zone owner.

// exwPickCands lists into d the exile cards matching the pick's filter.
func (s *State) exwPickCands(k *prep, in *kop.Instr, d *Decision) {
	for pp := uint8(0); pp < MaxSeats; pp++ {
		pl := &s.pl[pp]
		for _, o := range pl.exile[:pl.exileN] {
			if s.picksHas(o) {
				continue
			}
			if s.matchCardGen(k, in.F, s.objs[o].card, pp, s.res.e.ctrl, o) {
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			}
		}
	}
}

// randomExilePick is randomGravePick over the game-wide exile pool: gorge
// randomObjIDs takes a pool no larger than the count whole without a draw,
// else Rand(len(rest)) per pick with the pick removed. Nobody is asked.
//
//go:noinline
func (s *State) randomExilePick(k *prep) {
	in := &s.prog(k)[s.res.pc]
	var rest [2 * MaxDeck]Obj
	m := 0
	for pp := uint8(0); pp < MaxSeats; pp++ {
		pl := &s.pl[pp]
		for _, o := range pl.exile[:pl.exileN] {
			if s.matchCardGen(k, in.F, s.objs[o].card, pp, s.res.e.ctrl, o) {
				rest[m] = o
				m++
			}
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
