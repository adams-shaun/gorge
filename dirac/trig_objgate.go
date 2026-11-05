package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// objTrigGate is objTrigGen's cold gate over the flags it reads only when
// set: the turn gates (Tf2YourTurn, Tf2OppTurn) and Condition$ Evolve
// (Tf2Evolve: gorge evolveConditionHolds, the entering creature at evSl
// has greater power or greater toughness than the source, both read with
// their layers as the trigger fires).
//
//go:noinline
func (s *State) objTrigGate(k *prep, l trigListener, a *kop.Ability, evSl Slot) bool {
	f := a.Trig.Flags2
	if f&kop.Tf2YourTurn != 0 && s.active != l.ctrl || f&kop.Tf2OppTurn != 0 && s.active == l.ctrl {
		return false
	}
	if f&kop.Tf2Evolve != 0 {
		if evSl == NoSlot || l.sl == NoSlot {
			return false
		}
		ep, et := s.pt(k, evSl)
		sp, st := s.pt(k, l.sl)
		return ep > sp || et > st
	}
	return true
}

// batchOrProvOK is objTrigGen's cold gate for Tf2Batch and
// Tf2CastFromHand (one mask test on the hot path).
//
//go:noinline
func (s *State) batchOrProvOK(l trigListener, a *kop.Ability, o Obj) bool {
	if a.Trig.Flags2&kop.Tf2CastFromHand != 0 {
		e := s.resSpellOf(l.obj)
		if e == nil || !s.castFromHandBy(e, l.ctrl) {
			return false
		}
	}
	return a.Trig.Flags2&kop.Tf2Batch == 0 || s.batchTrigOK(l, a, o)
}
