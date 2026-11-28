package oracle

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
)

// The set atoms of the kernel's push-base kinds and combinators (kop
// setexpr.go): one atom per base kind and per combinator, each beside the
// gorge Defined$ spelling that names the same set.
func init() {
	b := func(k kop.SetOpKind, arg uint16) kop.SetOp { return kop.SetOp{Kind: k, Arg: arg} }
	for _, a := range []SetAtom{
		// bases
		{Name: "set:Targeted", GorgeDefined: "Targeted", Prog: kop.SetExpr{b(kop.SetTargets, 1<<kop.MaxTargets-1)}},
		{Name: "set:Self", GorgeDefined: "Self", Prog: kop.SetExpr{b(kop.SetSelf, 0)}},
		{Name: "set:You", GorgeDefined: "You", Prog: kop.SetExpr{b(kop.SetController, 0)}},
		{Name: "set:TriggeredCard", GorgeDefined: "TriggeredCard", Prog: kop.SetExpr{b(kop.SetTriggerObj, 0)},
			Bound: oneTriggerObject},
		{Name: "set:TriggeredPlayer", GorgeDefined: "TriggeredPlayer", Prog: kop.SetExpr{b(kop.SetTriggerPlayer, 0)},
			Bound: func(c *effects.Ctx) bool { return c.TriggerPlayer.IsPlayer }},
		{Name: "set:TriggeredSource", GorgeDefined: "TriggeredSource", Prog: kop.SetExpr{b(kop.SetTriggerSource, 0)},
			Bound: oneTriggerObject},
		{Name: "set:Opponent", GorgeDefined: "Opponent", Prog: kop.SetExpr{b(kop.SetEachOpponent, 0)}},
		{Name: "set:Player", GorgeDefined: "Player", Prog: kop.SetExpr{b(kop.SetEachPlayer, 0)}},
		// combinators
		{Name: "set:Targeted.Creature", GorgeDefined: "Targeted.Creature", Filters: []string{"Creature"},
			Prog: kop.SetExpr{b(kop.SetTargets, 1<<kop.MaxTargets-1), b(kop.SetAndFilter, 0)}},
		{Name: "set:Self & You", GorgeDefined: "Self & You", Unordered: true,
			Prog: kop.SetExpr{b(kop.SetSelf, 0), b(kop.SetController, 0), b(kop.SetUnion, 0)}},
		{Name: "set:NonYou", GorgeDefined: "NonYou",
			Prog: kop.SetExpr{b(kop.SetController, 0), b(kop.SetPlayersComplement, 0)}},
		// gorge's TargetedController reads the object targets only (a target
		// player contributes nothing), so the program drops the players with
		// an every-object filter before ControllerOf, which keeps a player as
		// its own controller (gorge's controllersOf).
		{Name: "set:TargetedController", GorgeDefined: "TargetedController", Unordered: true, Filters: []string{"Card"},
			Prog: kop.SetExpr{b(kop.SetTargets, 1<<kop.MaxTargets-1), b(kop.SetAndFilter, 0), b(kop.SetControllerOf, 0)}},
		{Name: "set:TriggeredCardOwner", GorgeDefined: "TriggeredCardOwner",
			Prog: kop.SetExpr{b(kop.SetTriggerObj, 0), b(kop.SetOwnerOf, 0)}, Bound: oneTriggerObject},
	} {
		RegisterSetAtom(a)
	}
}

// oneTriggerObject is whether gorge's trigger capture is the kernel's
// trigger object and source: an object event (a player event, which gorge binds a
// TriggerPlayer for, carries no object in the kernel: a Drawn trigger's
// capture is the drawn card in gorge and NoObj in the kernel) whose capture
// names at most one object. A capture of a group (a declare-attackers
// trigger's Remembered holds every attacker of the event, engine/rules
// triggerRemembered) reads as the whole group in gorge, where the kernel's
// trigger object is the one attacker (gorge's TriggeredAttacker). A lowering
// must not emit SetTriggerObj (TriggeredCard) or SetTriggerSource
// (TriggeredSource) on either kind of mode.
func oneTriggerObject(c *effects.Ctx) bool {
	if c.TriggerPlayer.IsPlayer {
		return false
	}
	n := 0
	for _, t := range c.Remembered {
		if !t.IsPlayer && t.Obj != 0 {
			n++
		}
	}
	return n <= 1
}
