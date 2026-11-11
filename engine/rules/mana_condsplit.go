package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// condSplitLiveSub is the production a Condition-split mana ability ma of
// source id would make instead of its own on the live board, or nil.
//
// Condition-split: ma is a Condition*-gated mana ability whose SubAbility$
// chain reaches a Condition*-gated DB$ Mana (Gemstone Caverns' "{T}: Add
// {C}. If it has a luck counter on it, instead add one mana of any color";
// River of Tears' "{T}: Add {U}. If you played a land this turn, add {B}
// instead"). The activation makes the root's mana or the sub's per the
// board at activation (CR 605.1a, 106.1), so the readers that bound what a
// source can make -- PotentialMana's fold (potentialManaFor) and the
// relaxed unpayability proof (paymentPlanRelaxedAlternatives) -- must count
// the sub's production too, or they under-produce and hide a payable play.
//
// The sub is returned when its gate holds now, or when the evaluator cannot
// decide it (an upper bound counts what might be made); an arm whose gate
// provably fails is not counted, so an unlucky Gemstone stays {C}. The
// root's own production is always counted by the caller: counting it beside
// the sub only over-produces.
func (e *Engine) condSplitLiveSub(p state.PlayerID, id state.ObjID, ma *cards.SA) *cards.SA {
	if ma == nil || ma.API != "Mana" || !effects.ActivationOf(ma).Cond.Any() {
		return nil
	}
	sub := cards.ManaChainProduction(ma)
	if sub == nil || !effects.ActivationOf(sub).Cond.Any() {
		return nil
	}
	ctx := e.manaAmountCtx(p, id)
	if o := e.G.Obj(id); o != nil && o.Face() != nil {
		effects.SetSVars(ctx, o.Face().SVars)
	}
	if met, resolved := effects.ConditionMet(e, ctx, sub); met || !resolved {
		return sub
	}
	return nil
}
