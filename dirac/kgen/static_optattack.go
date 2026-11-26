package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// S:Mode$ OptionalAttackCost | ValidCard$ Card.Self | Cost$ Exert<1/CARDNAME>
// [| Trigger$ <SVar>] ("you may exert CARDNAME as it attacks. When you do,
// ...", CR 702.100a; gorge engine/rules/combat.go exertOfferHolds,
// startExertAsks, trigger_match.go checkExertTriggers). The static is the
// offer: each declared attacker carrying it is asked, after the band asks and
// before the priority round, whether to exert (dirac/combat_exert.go); a yes
// pays Exert<1/CARDNAME> (the no-untap mark) and queues the rider. gorge
// queues the rider off the permanent's own Exert event, so it lowers as the
// card's own self-Exerted trigger over the Trigger$ body (the kernel fires it
// from the same yes), never as a printed Mode$ Exerted line (lc.exertRider).
//
// Only the shape gorge offers lowers: ValidCard$ Card.Self and the Exert<1/
// CARDNAME> cost (any other cost fails closed in gorge), with no gate
// (IsPresent$, Condition$, CheckSVar$: gorge reads them through
// continuousGateHolds, which the kernel does not mirror for this offer; the
// corpus's one gated carrier, Combat Celebrant, also reads notExertedThisTurn)
// and a rider with no target (a granted-trigger placement's target
// announcement is not proven against the kernel's).
func init() {
	staticMode("OptionalAttackCost", (*staticCtx).modeOptionalAttackCost)
}

func (sc *staticCtx) modeOptionalAttackCost() ([]lStatic, bool, error) {
	lc, sp := sc.lc, sc.sp
	if sc.st.Cond.Kind != cNone {
		return nil, true, refuse("OptionalAttackCost with a Condition$ / IsPresent$ / CheckSVar$ gate")
	}
	if v := sp.str("ValidCard"); v != "Card.Self" {
		return nil, true, refuse("OptionalAttackCost ValidCard %q", v)
	}
	if v := sp.str("Cost"); v != "Exert<1/CARDNAME>" {
		return nil, true, refuse("OptionalAttackCost Cost %q", v)
	}
	sp.consume("ValidCard", "Cost")
	exec, ok := sp.get("Trigger")
	if !ok {
		return nil, false, nil // a bare exert: the offer alone
	}
	sp.consume("Trigger")
	body := cards.ResolveSVar(lc.face.SVars, exec)
	if body == nil {
		return nil, true, refuse("OptionalAttackCost Trigger$ %q has no body", exec)
	}
	t := cards.Trigger{Mode: "Exerted", Params: map[string]string{
		"Mode": "Exerted", "ValidCard": "Card.Self", "Execute": exec,
		"TriggerDescription": sp.str("Description"),
	}, Effect: body}
	idx := len(lc.out.Abilities)
	lc.exertRider = true
	err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") })
	lc.exertRider = false
	if err != nil {
		return nil, true, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return nil, true, refuse("OptionalAttackCost rider lowering")
	}
	if len(lc.out.Abilities[idx].Targets) != 0 {
		return nil, true, refuse("OptionalAttackCost rider with a target")
	}
	return nil, false, nil
}

// optionalAttackCostAbility builds the offer static (kop.StatOptionalAttackCost):
// no filter, since the offer's source is the attacker itself.
func (cb *cardBuilder) optionalAttackCostAbility(s lStatic) (kop.Ability, []uint8, error) {
	ka := kop.Ability{Kind: kop.AbStatic}
	ka.Static.Kind = kop.StatOptionalAttackCost
	return ka, nil, nil
}
