package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// noRegenRemembered lowers the follow-up Effect of a damage spell that says
// "a creature dealt damage this way can't be regenerated this turn"
// (Incinerate): a CantRegenerate static over the creatures the previous
// DealDamage remembered (RememberDamaged$), ended for each as it leaves the
// battlefield. The mark is set where the damage lands (kernel
// DmgNoRegen, execDamageEx), exactly on a creature dealt damage above 0, so
// this node emits no instruction of its own: it folds into the DealDamage
// just emitted and reports handled. Any other shape is not this form.
func (lc *lowerCtx) noRegenRemembered(ps *params, ss []cards.Static) (bool, error) {
	if len(ss) != 1 || ss[0].Mode != "CantRegenerate" {
		return false, nil
	}
	switch ps.str("RememberObjects") {
	case "Remembered.Creature":
	case "ParentTarget":
		return lc.noRegenParentTarget(ps, ss)
	default:
		return false, nil
	}
	_, hasDur := ps.m["Duration"]
	_, hasTgt := ps.m["ValidTgts"]
	if ps.str("ForgetOnMoved") != "Battlefield" || hasDur || hasTgt {
		return false, refuse("Effect CantRegenerate shape")
	}
	sp := newParams(ss[0].Params)
	sp.consume("Mode", "Description")
	if sp.str("ValidCard") != "Card.IsRemembered" || len(sp.unread()) != 0 {
		return false, refuse("Effect CantRegenerate ValidCard")
	}
	n := len(lc.ab.Prog)
	if n == 0 || lc.ab.Prog[n-1].Op != "DealDamage" || !strings.Contains(lc.ab.Prog[n-1].CS, "Remember") {
		return false, refuse("Effect CantRegenerate without the previous RememberDamaged$ damage")
	}
	lc.ab.Prog[n-1].CS += "|NoRegen"
	ps.consume("RememberObjects", "ForgetOnMoved", "StaticAbilities", "IsCurse")
	return true, nil
}

// noRegenParentTarget lowers the follow-up Effect of a one-target damage
// spell that says "it can't be regenerated this turn" (Rage of Purphoros):
// RememberObjects$ ParentTarget names the parent link's chosen creature (gorge
// effectRemembered), so the CantRegenerate static covers it whether or not
// the damage was prevented. The mark is the damage instruction's
// DmgNoRegenTarget flag, set on the target once the damage step ran (CR
// 701.19c; kernel execDamageEx), so the node emits nothing of its own. The
// parent link must be the one-creature-target DealDamage just emitted; any
// other shape refuses by name.
func (lc *lowerCtx) noRegenParentTarget(ps *params, ss []cards.Static) (bool, error) {
	_, hasDur := ps.m["Duration"]
	_, hasTgt := ps.m["ValidTgts"]
	if ps.str("ForgetOnMoved") != "Battlefield" || hasDur || hasTgt {
		return false, refuse("Effect CantRegenerate shape")
	}
	sp := newParams(ss[0].Params)
	sp.consume("Mode", "Description")
	if sp.str("ValidCard") != "Card.IsRemembered" || len(sp.unread()) != 0 {
		return false, refuse("Effect CantRegenerate ValidCard")
	}
	var parent *cards.SA
	for n := lc.chainRoot; n != nil && lc.curNode != nil; n = n.Sub {
		if n.Sub == lc.curNode {
			parent = n
		}
	}
	n := len(lc.ab.Prog)
	if parent == nil || parent.API != "DealDamage" || n == 0 || lc.ab.Prog[n-1].Op != "DealDamage" {
		return false, refuse("Effect CantRegenerate ParentTarget without the previous DealDamage")
	}
	in := &lc.ab.Prog[n-1]
	if in.Obj.Kind != rTarget || in.Obj.N < 0 || in.Obj.N >= len(lc.ab.Targets) {
		return false, refuse("Effect CantRegenerate ParentTarget without a damage target")
	}
	t := lc.ab.Targets[in.Obj.N]
	if t.F == nil || t.F.Players != 0 || !t.Max.isConst() || t.Max.N != 1 || t.Divided != 0 || strings.Contains(in.CS, "Divided") || strings.HasPrefix(in.Note, "damage map") {
		return false, refuse("Effect CantRegenerate ParentTarget beyond one creature target")
	}
	in.CS = strings.TrimPrefix(in.CS+"|NoRegenTarget", "|")
	ps.consume("RememberObjects", "ForgetOnMoved", "StaticAbilities", "IsCurse")
	return true, nil
}
