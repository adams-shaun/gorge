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

// DebugPlannerFlexCensus (TestPlannerRankParityMutation) restores the
// pre-fix payment-parity behaviour: plainListedPays prices a plain play with
// payDescNow instead of the play's own payment descriptor (so a restricted
// alternative the play admits drops from the census and the play falls to the
// script prefix), and an Improvise cast whose announcements-beside-a-credit
// do not lower the composed price falls back to the manual window.
// TestPlannerRankParityMutation sets it and requires the recorded divergences
// back.
var DebugPlannerFlexCensus bool

// DebugXWindowSkipLifeOnly (TestXWindowLifeOnlySourceMutation): the {X}
// witness census skips a life-only mana source (Treasonous Ogre), as it did
// when availUngranted's tap gate ran before the pain arm.
var DebugXWindowSkipLifeOnly bool

// activationsOfSource is ExActivations N 1: the evaluating source's non-mana
// activations this turn, the ActivationLimit$ N offer gate (gorge
// legal_loyalty.go activationLimitBlocked reads activationUsedCount before
// the activation is pushed, so the one being offered is not in it). kgen
// lowers it only on a printed battlefield ability that is its card's one
// non-mana activated ability, where the per-object census is the ability's
// own count (dirac/kgen/lower_activationlimitn.go). The census also counts a
// granted ability's activations, which gorge keeps under that grant's own
// count, so those (act_grantledger.go) come off.
func (s *State) activationsOfSource(ctx evalCtx) int {
	if ctx.src == NoObj {
		return 0
	}
	ob := &s.objs[ctx.src]
	if ob.zone != kop.ZoneBattlefield {
		return 0
	}
	return int(ob.activations()) - s.grantActsOf(ctx.src)
}
