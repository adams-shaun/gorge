package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

func init() { apiOp("ChangeCombatants", (*lowerCtx).changeCombatants) }

// ChangeCombatants lowers api:ChangeCombatants's Attacking$ True shape
// (gorge engine/effects/changecombatants.go effChangeCombatants): the
// resolving controller reselects which player each defined attacking
// creature attacks, one ask per attacker, and a pick that MOVES the attack
// unblocks it (gorge's CombatRetarget). The lowered subject is the effect's
// ValidTgts$ (Misleading Signpost, Portal Mage: one attacking creature) or a
// Defined$ Valid filter (every battlefield permanent matching it); gorge's
// Defined walk then keeps only a creature actually attacking in its
// controller's own combat.
//
// Every other shape refuses by name, naming gorge's Note or stand-in: another
// Attacking$ value (Capricopian's Player.OpponentOf CardController,
// Portal Manipulator's TargetedPlayer, Tahngarth's .Defending form) resolves
// as a loud Note and no move, and Optional$ True (Tuvok, Windshaper Planetar)
// poses a per-attacker may-reselect confirm the kernel does not lower. A Defined$ selector other than Valid
// (Kari Zev / Stormforged Armor's Remembered) is unimplemented in the kernel,
// not a gorge Note, so it refuses on its own reason.
func (lc *lowerCtx) changeCombatants(sa *cards.SA, ps *params) error {
	if v, ok := ps.get("Optional"); ok {
		if strings.TrimSpace(v) == "True" {
			return refuse("ChangeCombatants Optional$ True: the per-attacker may-reselect confirm is not lowered")
		}
		return refuse("ChangeCombatants Optional$ %q", strings.TrimSpace(v))
	}
	mode, _ := ps.get("Attacking")
	mode = strings.TrimSpace(mode)
	if mode != "True" {
		return refuse("ChangeCombatants Attacking$ %s: gorge resolves it as a Note", mode)
	}
	if d, ok := ps.get("Defined"); ok {
		d = strings.TrimSpace(d)
		if !strings.HasPrefix(d, "Valid ") {
			return refuse("ChangeCombatants Defined$ %q: only a Valid filter lowers", d)
		}
		f, err := lc.filter(strings.TrimSpace(strings.TrimPrefix(d, "Valid ")))
		if err != nil {
			return err
		}
		if f.Players != 0 || len(f.Clauses) == 0 {
			return refuse("ChangeCombatants Defined$ %q: a player alternative", d)
		}
		lc.emit(lInstr{Op: "ChangeCombatants", Obj: lRef{Kind: rNone}, F: f})
		return nil
	}
	ref, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	if ref.Kind == rTarget {
		t := lc.ab.Targets[ref.N]
		if !t.Max.isConst() || t.Max.N != 1 || t.F == nil || t.F.Players != 0 {
			return refuse("ChangeCombatants of a target slot other than one object")
		}
	}
	lc.emit(lInstr{Op: "ChangeCombatants", Obj: ref})
	return nil
}
