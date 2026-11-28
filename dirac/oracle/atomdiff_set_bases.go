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
		{Name: "set:TriggeredCard", GorgeDefined: "TriggeredCard", Prog: kop.SetExpr{b(kop.SetTriggerObj, 0)}},
		{Name: "set:TriggeredPlayer", GorgeDefined: "TriggeredPlayer", Prog: kop.SetExpr{b(kop.SetTriggerPlayer, 0)},
			Bound: func(c *effects.Ctx) bool { return c.TriggerPlayer.IsPlayer }},
		{Name: "set:TriggeredSource", GorgeDefined: "TriggeredSource", Prog: kop.SetExpr{b(kop.SetTriggerSource, 0)}},
		{Name: "set:Opponent", GorgeDefined: "Opponent", Prog: kop.SetExpr{b(kop.SetEachOpponent, 0)}},
		{Name: "set:Player", GorgeDefined: "Player", Prog: kop.SetExpr{b(kop.SetEachPlayer, 0)}},
		// combinators
		{Name: "set:Targeted.Creature", GorgeDefined: "Targeted.Creature", Filters: []string{"Creature"},
			Prog: kop.SetExpr{b(kop.SetTargets, 1<<kop.MaxTargets-1), b(kop.SetAndFilter, 0)}},
		{Name: "set:Self & You", GorgeDefined: "Self & You", Unordered: true,
			Prog: kop.SetExpr{b(kop.SetSelf, 0), b(kop.SetController, 0), b(kop.SetUnion, 0)}},
		{Name: "set:NonYou", GorgeDefined: "NonYou",
			Prog: kop.SetExpr{b(kop.SetController, 0), b(kop.SetPlayersComplement, 0)}},
		{Name: "set:TargetedController", GorgeDefined: "TargetedController", Unordered: true,
			Prog: kop.SetExpr{b(kop.SetTargets, 1<<kop.MaxTargets-1), b(kop.SetControllerOf, 0)}},
		{Name: "set:TriggeredCardOwner", GorgeDefined: "TriggeredCardOwner",
			Prog: kop.SetExpr{b(kop.SetTriggerObj, 0), b(kop.SetOwnerOf, 0)}},
	} {
		RegisterSetAtom(a)
	}
}
