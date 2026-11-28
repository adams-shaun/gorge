package oracle

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
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
		// gorge sweeps seat by seat in entry order, the kernel in index order
		{Name: "set:Valid Creature.YouCtrl", GorgeDefined: "Valid Creature.YouCtrl", Unordered: true, Filters: []string{"Creature.YouCtrl"},
			Prog: kop.SetExpr{b(kop.SetBattlefield, 0), b(kop.SetAndFilter, 0)}},
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

// RefBoard is the generated context a SetRefBase atom is read in, bound on
// both engines: the source and its controller, one object the object
// referents name and one seat the player referents name.
type RefBoard struct {
	Src, Obj state.ObjID
	Ctrl     state.PlayerID
	Player   state.PlayerID
}

// SetRefAtom is the atom of one admitted SetRefBase Ref (kop.SetRefBaseOK) or
// of the battlefield base: the kernel program beside gorge's Defined$ spelling
// of the same set. Bind binds b's referent into gorge's context as the event
// or choice that binds it would (the kernel side binds every referent at
// once, dirac.ProbeRefBinding), and reports false where the spelling reads
// nothing the Ref models (nil: the spelling reads the source alone).
type SetRefAtom struct {
	SetAtom
	Ref  kop.Ref // RefNone for a battlefield atom
	Bind func(h effects.Host, c *effects.Ctx, b RefBoard) bool
}

// SetRefAtoms is one atom per admitted Ref, in Ref order, the owner
// composition kgen narrows to a live attachment (lower_definedset.go
// setStillAttached), and the battlefield base reading the source.
func SetRefAtoms() []SetRefAtom {
	ref := func(r kop.Ref, defined string, bind func(effects.Host, *effects.Ctx, RefBoard) bool) SetRefAtom {
		return SetRefAtom{Ref: r, Bind: bind, SetAtom: SetAtom{Name: "setref:" + defined, GorgeDefined: defined,
			Prog: kop.SetExpr{{Kind: kop.SetRefBase, Arg: uint16(r)}}}}
	}
	// bound is a referent every board binds through one context field
	bound := func(set func(c *effects.Ctx, b RefBoard)) func(effects.Host, *effects.Ctx, RefBoard) bool {
		return func(_ effects.Host, c *effects.Ctx, b RefBoard) bool { set(c, b); return true }
	}
	obj := func(b RefBoard) state.Target { return state.Target{Obj: b.Obj} }
	player := func(b RefBoard) state.Target { return state.Target{Player: b.Player, IsPlayer: true} }
	remembered := bound(func(c *effects.Ctx, b RefBoard) { c.Remembered = []state.Target{obj(b)} })
	chosen := func(t func(RefBoard) state.Target) func(effects.Host, *effects.Ctx, RefBoard) bool {
		return bound(func(c *effects.Ctx, b RefBoard) { c.Chosen, c.ChosenValid = []state.Target{t(b)}, true })
	}
	// The last bearer of a source that left is read from records the kernel
	// keeps only as long as a pending ability can need them (two beyond the
	// departed slot, dirac static_lastbearer.go), not for any old graveyard
	// card, so the two LKI reads are compared for a source on the battlefield.
	standing := func(h effects.Host, c *effects.Ctx, _ RefBoard) bool {
		o := h.Game().Obj(c.Source)
		return o != nil && o.Zone == state.ZBattlefield
	}
	owner := ref(kop.RefAttached, "EnchantedOwner", nil)
	owner.Filters = []string{"Card.EnchantedBy"}
	owner.Prog = append(owner.Prog, kop.SetOp{Kind: kop.SetAndFilter}, kop.SetOp{Kind: kop.SetOwnerOf})
	return []SetRefAtom{
		ref(kop.RefSelf, "Self", nil),
		ref(kop.RefController, "You", nil),
		ref(kop.RefTriggerObj, "TriggeredCard", remembered),
		ref(kop.RefTriggerPlayer, "TriggeredPlayer", bound(func(c *effects.Ctx, b RefBoard) { c.TriggerPlayer = player(b) })),
		ref(kop.RefAttached, "Enchanted", standing),
		ref(kop.RefDefendingPlayer, "TriggeredDefendingPlayer", bound(func(c *effects.Ctx, b RefBoard) { c.DefendingPlayer = player(b) })),
		ref(kop.RefTriggerSource, "TriggeredSource", bound(func(c *effects.Ctx, b RefBoard) { c.TriggerSource = b.Obj })),
		ref(kop.RefTriggerTgtCtrl, "TriggeredTargetController", bound(func(c *effects.Ctx, b RefBoard) { c.TriggerTarget = obj(b) })),
		// the blocker a Blocks trigger over the attached creature captures is
		// that creature; an unattached source has none
		ref(kop.RefAttachedCtrl, "TriggeredBlockerController", func(h effects.Host, c *effects.Ctx, b RefBoard) bool {
			host := effects.DefinedSpec(h, c, "Enchanted")
			if len(host) != 1 || !standing(h, c, b) {
				return false
			}
			c.TriggerBlocker = host[0].Obj
			return true
		}),
		ref(kop.RefTriggerObjOwner, "TriggeredCardOwner", bound(func(c *effects.Ctx, b RefBoard) { c.TriggerCard = b.Obj })),
		ref(kop.RefResChosenPlayer, "ChosenPlayer", chosen(player)),
		ref(kop.RefEnchantedCtrl, "EnchantedController", nil),
		owner,
		{SetAtom: SetAtom{Name: "setref:Valid Card.EnchantedBy", GorgeDefined: "Valid Card.EnchantedBy", Unordered: true,
			Filters: []string{"Card.EnchantedBy"}, Prog: kop.SetExpr{{Kind: kop.SetBattlefield}, {Kind: kop.SetAndFilter}}}},
	}
}
