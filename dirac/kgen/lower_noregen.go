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
	if len(ss) != 1 || ss[0].Mode != "CantRegenerate" || ps.str("RememberObjects") != "Remembered.Creature" {
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
