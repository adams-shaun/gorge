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
	case "Remembered.Creature", "Remembered":
		// (Jaya Ballard, Task Mage) gorge effectRememberedCodes maps both
		// spellings to Remembered: the dealt set the previous
		// RememberDamaged$ damage holds
	case "ParentTarget":
		return lc.noRegenParentTarget(ps, ss)
	case "Targeted":
		// (Engulfing Flames) an Effect with no target of its own: the chain's
		// target is the parent link's chosen creature, the same set
		// RememberObjects$ ParentTarget names (gorge effectRemembered). A
		// targeted Effect with its own ValidTgts$ names ITS target instead
		// and is not this shape.
		if _, hasOwn := ps.m["ValidTgts"]; !hasOwn {
			return lc.noRegenParentTarget(ps, ss)
		}
		return false, nil
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

// noRegenMark lowers a targeted (or remembered) Effect whose one static is
// Mode$ CantRegenerate with no DealDamage carrying the mark (Furnace Brood,
// Hurr Jackal, Orcish Healer, Gravebind: "target creature can't be
// regenerated this turn"): the kernel's OpMarkNoRegen takes the mechNoRegen
// slot mark at resolution (dirac/act_noregenmark.go), ending at the turn's
// cleanup and the creature's departure. gorge registers the restriction over
// the remembered id and forgets it when the card moves (ForgetOnMoved$
// Battlefield), so the mark requires that rider: without it a departed and
// returned target would come back restricted in gorge but not in the kernel
// (CR 400.7: a new object), and a Duration$ other than the mark's this-turn
// life has no kernel record.
func (lc *lowerCtx) noRegenMark(ps, sp *params, obj lRef) error {
	if f, ok := ps.m["ForgetOnMoved"]; !ok || strings.TrimSpace(f) != "Battlefield" {
		return refuse("Effect CantRegenerate shape")
	}
	if _, ok := ps.m["Duration"]; ok {
		return refuse("Effect CantRegenerate Duration$")
	}
	switch obj.Kind {
	case rTarget, rRemembered:
	default:
		return refuse("Effect CantRegenerate over %s", obj)
	}
	if sp.str("ValidCard") != "Card.IsRemembered" || len(sp.unread()) != 0 {
		return refuse("Effect CantRegenerate ValidCard")
	}
	ps.consume("IsCurse", "ForgetOnMoved")
	lc.emit(lInstr{Op: "NoRegenMark", Obj: obj, Note: "this turn: the creature can't be regenerated"})
	return nil
}
