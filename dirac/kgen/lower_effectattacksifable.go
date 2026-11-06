package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// attackBlockPair reports whether ss is exactly the two statics
// MustAttack and MustBlock, in either order (Hustle's "attacks or blocks
// this turn if able": StaticAbilities$ MustAttack,MustBlock over one
// remembered target).
func attackBlockPair(ss []cards.Static) bool {
	return len(ss) == 2 && (ss[0].Mode == "MustAttack" && ss[1].Mode == "MustBlock" ||
		ss[0].Mode == "MustBlock" && ss[1].Mode == "MustAttack")
}

// attacksOrBlocksIfAble lowers the MustAttack,MustBlock pair: both duties
// on the one object obj, the MustAttack static first (so its Duration$
// refusal names the shape). Each static goes through effectTargetStatic,
// as a one-static Effect's does.
func (lc *lowerCtx) attacksOrBlocksIfAble(ps *params, ss []cards.Static, obj lRef) error {
	for _, mode := range [...]string{"MustAttack", "MustBlock"} {
		for _, s := range ss {
			if s.Mode != mode {
				continue
			}
			if err := lc.effectTargetStatic(ps, s, obj); err != nil {
				return err
			}
		}
	}
	return nil
}

// attacksIfAble lowers an Effect's Mode$ MustAttack static: "target
// creature attacks this turn if able" (CR 508.1d; Heckling Fiends,
// Incite, Courtly Provocateur). gorge effEffect registers the static on the
// Effect over its remembered target; the kernel marks that target
// (ExtAttacksIfAble, dirac/combat_attacksifable.go) and reads the mark beside
// the MustAttack static. Only the exact shape: the single remembered target,
// the default this-turn lifetime, the target leaving ending it.
func (lc *lowerCtx) attacksIfAble(ps, sp *params, obj lRef) error {
	if v, ok := sp.get("MustAttack"); ok {
		// the mark has no defender: "attacks YOU if able" (Alluring Siren)
		return refuse("Effect static MustAttack$ %q", v)
	}
	vc := sp.str("ValidCreature")
	if vc == "Creature.RememberedPlayerCtrl" {
		// every creature a remembered player controls (Imaginary Threats,
		// Taunt): a player-scoped requirement, not a per-object mark
		return refuse("Effect MustAttack player scope %q", vc)
	}
	if obj.Kind != rTarget || vc != "Card.IsRemembered" && vc != "Creature.IsRemembered" ||
		ps.str("ExileOnMoved") != "Battlefield" {
		return refuse("Effect MustAttack shape")
	}
	if d, ok := ps.m["Duration"]; ok {
		// the mark lasts this turn; the kernel has no extra combat to end
		// an UntilEndOfCombat one early (Boros Battleshaper)
		return refuse("Effect MustAttack Duration$ %q", d)
	}
	ps.consume("ExileOnMoved")
	lc.emit(lInstr{Op: "AttacksIfAble", Obj: obj, Note: "the target attacks this turn if able"})
	return nil
}

// blocksIfAble lowers an Effect's Mode$ MustBlock static: "target creature
// blocks this turn if able" (CR 509.1c; Culvert Ambusher, Courtly Provocateur,
// Hustle // Bustle). gorge effEffect registers the static on the Effect over
// its remembered target; the kernel marks that target (ExtBlocksIfAble,
// dirac/combat_blocksifable.go) and combat reads the mark as a blocker
// requirement beside the MustBlock static. Only the exact shape: the single
// remembered target, the default this-turn lifetime, the target leaving
// ending it.
func (lc *lowerCtx) blocksIfAble(ps, sp *params, obj lRef) error {
	vc := sp.str("ValidCreature")
	if obj.Kind != rTarget || vc != "Card.IsRemembered" && vc != "Creature.IsRemembered" ||
		ps.str("ExileOnMoved") != "Battlefield" {
		return refuse("Effect MustBlock shape")
	}
	if _, ok := ps.m["Duration"]; ok {
		// the mark lasts this turn; the kernel has no extra combat to end
		// an UntilEndOfCombat one early
		return refuse("Effect MustBlock Duration$ %q", ps.str("Duration"))
	}
	ps.consume("ExileOnMoved")
	lc.emit(lInstr{Op: "BlocksIfAble", Obj: obj, Note: "the target blocks this turn if able"})
	return nil
}
