package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// activationsOfResolving is ExActivations: the resolving activated ability's
// source's non-mana activations this turn, the resolving one included (gorge
// ConditionActivationLimit$, Ctx.ActivationsThisTurn = activationUsedCount).
// The census (object.acts) counts the activation as pushBuild pushes it, so an
// activation still on the stack is in it too, and it ends with the stint
// (zones.go: CR 400.7). A source that has left the battlefield, or is back as
// a new object, reads 0: the ability then has no source to count, and the
// effects gate fails the condition.
//
//go:noinline
func (s *State) activationsOfResolving() int {
	if !s.res.active || DebugNoActCount {
		return 0
	}
	e := &s.res.e
	if e.kind != seAbility {
		return 0
	}
	ob := &s.objs[e.obj]
	if ob.ver != e.srcVer || ob.zone != kop.ZoneBattlefield {
		return 0
	}
	return int(ob.activations())
}

// DebugNoActCount (TestActivationLimitMutation): ExActivations reads 0, so a
// ConditionActivationLimit$ gate never holds.
var DebugNoActCount bool
