package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// mustBlockAPI lowers api:MustBlock, "<blocker> blocks CARDNAME this turn if
// able" (CR 509.1c; gorge effMustBlock, engine/effects/mustblock_api.go: one
// MustBlock continuous duty per blocker, bound to the attacker frozen at
// resolution, MustBlockAttacker, read per pair by
// combat.MustBlockPairRequired) to OpExt ExtMustBlock: the blocker Obj must
// block the resolving source if able, this turn (C kop.DelayEndCombat: until
// the end of this combat). The kernel records the (blocker, attacker) pair
// (dirac/combat_mustblockatk.go) and counts it in the same
// maximum-requirements search as the other block requirements: one unit per
// duty blocker that blocks an attacker it is bound to.
//
// Two blocker forms lower. A single target slot (ValidTgts$: Avalanche
// Tusker, Tower Above's granted trigger). Crashing Boars' choice pool
// (Choices$ Creature.untapped+DefenderCtrl | Chooser$ TriggeredDefendingPlayer,
// gorge effMustBlockChoicePool): the defending player picks one of their
// untapped creatures (ChooseCard by the defender) and the pick is the
// blocker; gorge asks only while the source is on the battlefield, so the
// pick is skipped otherwise. The attacker is the source (no DefinedAttacker$);
// a DefinedAttacker$, a Defined$ blocker, BlockAllDefined$ and any other
// Duration$ refuse by name.
func (lc *lowerCtx) mustBlockAPI(ps *params) error {
	for _, k := range []string{"DefinedAttacker", "Defined", "BlockAllDefined", "TargetMin", "TargetMax"} {
		if v, ok := ps.m[k]; ok {
			return refuse("MustBlock %s$ %q", k, v)
		}
	}
	c := 0
	if d, ok := ps.get("Duration"); ok {
		if strings.TrimSpace(d) != "UntilEndOfCombat" {
			return refuse("MustBlock Duration$ %q", d)
		}
		c = kop.DelayEndCombat
	}
	if ch, ok := ps.get("Choices"); ok {
		if strings.TrimSpace(ch) != "Creature.untapped+DefenderCtrl" || ps.str("Chooser") != "TriggeredDefendingPlayer" {
			return refuse("MustBlock Choices$ %q Chooser$ %q", ch, ps.str("Chooser"))
		}
		if _, ok := ps.m["ValidTgts"]; ok {
			return refuse("MustBlock Choices$ with ValidTgts$")
		}
		ps.consume("Chooser")
		f, err := lc.filter(ch)
		if err != nil {
			return err
		}
		// SkipIf skips unless its condition holds (kernel OpSkipUnless)
		lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cSourceZone, Zone: uint8(zBattlefield)}, Skip: 2,
			Note: "gorge asks only while the attacker is on the battlefield"})
		lc.emit(lInstr{Op: "ChooseCard", Obj: lRef{Kind: rDefending}, F: f, C: zBattlefield, Note: "the defending player chooses the blocker"})
		lc.emit(lInstr{Op: "MustBlockAttacker", Obj: lRef{Kind: rChosen}, C: c, Note: "the chosen creature blocks the source if able"})
		return nil
	}
	if _, ok := ps.m["ValidTgts"]; !ok {
		return refuse("MustBlock without ValidTgts$ or Choices$")
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget {
		return refuse("MustBlock blocker %s", obj)
	}
	if t := lc.ab.Targets[obj.N]; !t.Max.isConst() || t.Max.N != 1 || t.F == nil || t.F.Players != 0 {
		return refuse("MustBlock of a target slot other than one object")
	}
	lc.emit(lInstr{Op: "MustBlockAttacker", Obj: obj, C: c, Note: "the target blocks the source if able"})
	return nil
}
