package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a printed graveyard ability's "activate only if"
// (gorge activationConditionOK / abilityPresentHolds, evaluated in the one
// offer walk for every zone). kgen lowers only a condition that reads the
// ACTIVATOR's zones or turn (graveActCondOK), so the evaluation needs no
// source slot: countPerm and the graveyard/exile counts read ctx.ctrl and
// the filter, never ctx.srcSl.

// DebugGraveActNoCond is TestActgraveMutation's switch: the graveyard offer
// ignores the activation condition.
var DebugGraveActNoCond bool

// graveActCond reports whether the activation condition of graveyard
// ability a (Trig.Cond, zero when none) holds for p, the card o's owner.
func (s *State) graveActCond(k *prep, a *kop.Ability, p uint8, o Obj) bool {
	if DebugGraveActNoCond {
		return true
	}
	return a.Trig.Cond == 0 || s.condEval(k, a.Trig.Cond, evalCtx{src: o, srcSl: NoSlot, ctrl: p})
}
