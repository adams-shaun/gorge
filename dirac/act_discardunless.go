package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ChGenDiscUnless is OpDiscardUnless's election (gorge discard_unless):
// mode 1 = unless (discard one matching card), 0 = ordinary. A high value,
// clear of the siblings' appended kinds.
const ChGenDiscUnless ChoiceKind = 200

// discardUnless is OpDiscardUnless (gorge effDiscard's TgtChoose arm with
// UnlessType$; Thirst for Discovery): with no card matching F in the
// discarding player's hand it is the ordinary discard; otherwise the
// player elects unless / ordinary first (discard_unless).
func (s *State) discardUnless(k *prep, in *kop.Instr) bool {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return true
	}
	if _, n := s.unlessMatch(k, in, p); n == 0 {
		return s.discardOrdinary(k, in, p)
	}
	s.res.ask, s.res.askP, s.res.chosen = raGenDiscUnless, p, objWord(p)
	return false
}

// unlessMatch counts player p's hand cards matching the op's unless filter
// and returns the first.
func (s *State) unlessMatch(k *prep, in *kop.Instr, p uint8) (Obj, int) {
	pl := &s.pl[p]
	first, n := NoObj, 0
	for i := 0; i < int(pl.handN); i++ {
		if o := pl.hand[i]; s.discardMatch(k, in.F, o) {
			if n == 0 {
				first = o
			}
			n++
		}
	}
	return first, n
}

// discardOrdinary is the ordinary self-chosen discard of A cards (the core's
// OpDiscard ChooseSelf arm, gorge discardBounds): a resolved zero discards
// nothing and asks nothing; a hand no larger is discarded whole, else the
// raDiscard picks (false).
func (s *State) discardOrdinary(k *prep, in *kop.Instr, p uint8) bool {
	n := s.value(k, in.A)
	if n < 1 {
		return true
	}
	pl := &s.pl[p]
	if n >= int(pl.handN) {
		for pl.handN > 0 {
			s.discardObj(k, pl.hand[0])
		}
		return true
	}
	s.res.ask, s.res.peekI, s.res.askP = raDiscard, 0, p
	s.res.left, s.res.chosen = uint8(n), objWord(p)
	s.ctl = ctlResolve
	return false
}
