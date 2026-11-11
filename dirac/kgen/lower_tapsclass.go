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

// selfTapsPain reads a Self Taps trigger of a card with a mana ability as
// gorge's planner does (pay.PaymentPlanSelfDamageTrigger, spec 3.2's damage:N
// row): no trigger parameter beyond Mode, ValidCard, Execute, TriggerZones
// (Battlefield) and TriggerDescription, and a body that is exactly one
// DealDamage to Defined$ You with a literal NumDmg$ of at least 1 (tapsPlanClass's
// closed world). It returns the damage.
func selfTapsPain(t cards.Trigger) (uint8, bool) {
	for _, k := range slices.Sorted(maps.Keys(t.Params)) {
		switch k {
		case "Mode", "ValidCard", "Execute", "TriggerDescription":
		case "TriggerZones":
			if strings.TrimSpace(t.Params[k]) != "Battlefield" {
				return 0, false
			}
		default:
			return 0, false
		}
	}
	if t.Effect == nil || strings.TrimSpace(t.Effect.ParamStr(cards.PKDefined)) != "You" {
		return 0, false
	}
	if class, n := tapsPlanClass(t.Effect); class == kop.TapPlanPain {
		return n, true
	}
	return 0, false
}

// selfPainPlainMana reports whether mana ability m is exactly "{T}: Add one mana of
// <one colour, {C} or any color>" with no other parameter: the shape the kernel
// prices as a pain-only source when a self Pain-class Taps line rides it
// (dirac primaryTapAb).
func selfPainPlainMana(m *cards.SA) bool {
	if m.API != "Mana" {
		return false
	}
	for _, k := range slices.Sorted(maps.Keys(m.Params)) {
		switch k {
		case "API", "Cost", "Produced", "Amount", "SpellDescription":
		default:
			return false
		}
	}
	if strings.TrimSpace(m.ParamStr(cards.PKCost)) != "T" {
		return false
	}
	if a := strings.TrimSpace(m.ParamStr(cards.PKAmount)); a != "" && a != "1" {
		return false
	}
	switch strings.TrimSpace(m.ParamStr(cards.PKProduced)) {
	case "Any", "W", "U", "B", "R", "G", "C":
		return true
	}
	return false
}
