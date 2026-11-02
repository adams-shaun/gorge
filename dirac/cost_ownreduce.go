package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ownReduced returns activated ability a's mana cost after its own
// ReduceCost$ (kop.Ability.OwnReduce; Channel): the generic part
// less the value, floored at zero (gorge legal_activation.go ownReduceCost,
// folded identically by the offer gate, the planner witness and the
// charge). The value reads no target (kgen lower_ownreduce.go), so the
// offer and the charge agree. A source on the battlefield is read at its
// live permanent slot, so a `Count$CardCounters.<NAME>` body sees the
// permanent's counters exactly as gorge's obj-counter read does (Diary of
// Dreams' page counters); a card in any other zone has no counters, which
// is gorge's 0 too. The caller guards on anyOwnReduce; this body is cold.
//
//go:noinline
func (s *State) ownReduced(k *prep, a *kop.Ability, src Obj, ctrl uint8) kop.ManaCost {
	c := a.Cost.Mana
	if DebugNoOwnReduce {
		return c
	}
	srcSl := NoSlot
	if src != NoObj {
		if ob := &s.objs[src]; ob.zone == kop.ZoneBattlefield {
			srcSl = ob.slot
		}
	}
	n := s.evalV(k, a.OwnReduce, evalCtx{src: src, srcSl: srcSl, ctrl: ctrl})
	if n <= 0 {
		return c
	}
	if n >= int(c.Generic) {
		c.Generic = 0
	} else {
		c.Generic -= uint8(n)
	}
	return c
}

// DebugNoOwnReduce drops the reduction (the channel mutation test proves
// the pool activates a reduced channel ability).
var DebugNoOwnReduce bool

// ownReducedBF returns battlefield activated ability a's mana cost after
// its own ReduceCost$: gorge's battlefield offer
// (legal_walk_battlefield.go ownReduceCost), the planner witness and the
// charge all subtract the value from the generic part, floored at zero
// (CR 601.2f). The value reads no target (kgen lower_ownreduce.go); the
// source is the permanent's object and the controller is the activator.
// The caller guards on anyOwnReduce; this body is cold.
//
//go:noinline
func (s *State) ownReducedBF(k *prep, a *kop.Ability, sl Slot, ctrl uint8) kop.ManaCost {
	return s.ownReduced(k, a, s.perms[sl].obj, ctrl)
}
