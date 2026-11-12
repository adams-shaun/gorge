package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// stat:CantPayLife is the printed S:Mode$ CantPayLife static (Angel of
// Jubilation, Yasharn, Implacable Earth: "Players can't pay life or sacrifice
// creatures to cast spells or activate abilities"; Karn's Sylex). Its own init
// keeps the registration out of the shared statics.go list.
func init() { effects.RegisterNonAPI("stat:CantPayLife") }

// playerLifePayForbidden reports whether a CantPayLife static forbids p paying
// life in a payment of this kind: the cause-scoped half of the payment layer's
// pay.Eval.LifePaymentForbidden (the CantChangeLife/CantLoseLife half is
// player-wide and has no cause). Only the cost path reads it: ForCost$ False is
// an effect-path line this build has no reader for (inert here), and an absent
// ForCost$ applies. ValidCause$ is read through causeCostAdmits, the
// CantSacrifice cost path's evaluator, so a ward, upkeep or unless payment
// (SAOther) and an attack cost stay payable under `Spell,Activated`; a
// qualified alternative (Karn's Sylex's Activated.!ManaAbility) is skipped,
// failing closed (under-enforced). A free function over the Engine: payment
// logic takes no new Engine method.
func playerLifePayForbidden(e *Engine, p state.PlayerID, kind pay.SAKind) bool {
	for _, sv := range e.activeStatics("CantPayLife") {
		if !effects.CantPayLifeParamsReadable(sv.Params) {
			continue
		}
		if sacrificeBlockedCodes.Code(string(sv.ParamStr(cards.PKForCost))) == sacrificeBlockedFalse {
			continue
		}
		if spec := sv.ParamStr(cards.PKValidPlayer); spec != "" &&
			!effects.MatchesPlayerSpec(e.G, spec, p, sv.Controller) {
			continue
		}
		if spec := sv.ParamStr(cards.PKValidCause); spec != "" &&
			!causeCostAdmits(spec, costCauseOfKind(kind)) {
			continue
		}
		return true
	}
	return false
}

// costCauseOfKind maps a payment's SAKind onto the cost cause causeCostAdmits
// reads: only a spell cast and an activation are causes a cost payment names.
func costCauseOfKind(kind pay.SAKind) pay.CostCause {
	switch kind {
	case pay.SASpell:
		return pay.CostCauseSpell
	case pay.SAActivated:
		return pay.CostCauseActivated
	}
	return pay.CostCauseNone
}
