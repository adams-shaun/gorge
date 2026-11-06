package kgen

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// tapsPlanClass is the payment-planner class of a lowered non-Self Taps
// watcher (kop.TapPlan), mirroring gorge's per-source tier
// (pay.PaymentPlanAbilityTier via paymentPlanTapObservers): a vetted literal
// player-damage body is Ordinary when its referent cannot name the payer
// (Opponent, EachOpponent) and Pain when it can (You, TriggeredCardController:
// for the two shapes lowered here, YouCtrl and EquippedBy, the watcher's
// controller, the tapped permanent's controller and the payer coincide);
// every other body defers its tapped source. The closed world is
// pay.paymentPlanPlayerDamageBody's: one DealDamage with a literal NumDmg$ of
// at least 1, no SubAbility$, and no parameter outside the allow-list.
func tapsPlanClass(body *cards.SA) (kop.TapPlan, uint8) {
	if body == nil || body.API != "DealDamage" || strings.TrimSpace(body.ParamStr(cards.PKSubAbility)) != "" {
		return kop.TapPlanDefer, 0
	}
	for _, k := range slices.Sorted(maps.Keys(body.Params)) {
		switch k {
		case "API", "Defined", "NumDmg", "DamageSource", "SpellDescription", "StackDescription":
		default:
			return kop.TapPlanDefer, 0
		}
	}
	n, err := strconv.ParseUint(strings.TrimSpace(body.ParamStr(cards.PKNumDmg)), 10, 8)
	if err != nil || n < 1 {
		return kop.TapPlanDefer, 0
	}
	switch strings.TrimSpace(body.ParamStr(cards.PKDefined)) {
	case "You", "TriggeredCardController":
		return kop.TapPlanPain, uint8(n)
	case "Opponent", "EachOpponent":
		return kop.TapPlanOrdinary, 0
	}
	return kop.TapPlanDefer, 0
}
