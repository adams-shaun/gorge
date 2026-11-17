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

// tapsAttachedClass is the payment-planner class of the AttachedBy Taps
// watcher: a body whose only effect is to destroy the tapped permanent itself
// (Brink of Disaster's, Cryoshatter's) is a post-payment consequence gorge's
// planner vets (pay PaymentPlanTapDestroySelfTrigger), so the source stays an
// ordinary unit (kop.TapPlanOrdinary); an unvetted body defers, as gorge's
// planner defers any tap a non-damage watcher sees; a damage body (Psychic
// Venom's payer damage) has a payment tiering the kernel does not mirror yet,
// so the caller refuses it.
func tapsAttachedClass(t cards.Trigger) (kop.TapPlan, uint8, bool) {
	tp8, pn := tapsPlanClass(t.Effect)
	destroySelf := tp8 == kop.TapPlanDefer && tapsDestroySelf(t)
	if destroySelf {
		tp8 = kop.TapPlanOrdinary
	}
	return tp8, pn, destroySelf
}

// tapsAttachedClauses vets the clause table of a non-Self Taps watcher with no
// ValidPlayer$: every clause is a plain ownerless one over your own creature
// (Ctrl You, no flags, no AttachedBy) or, on the enchanted-permanent watcher
// (any Ctrl, the EquippedBy flag), carries a plan the payment planner can
// mirror — a deferring body or a vetted destroy-self one. An unvetted
// AttachedBy body vetoes the lowering.
func tapsAttachedClauses(f *lFilter, vc string, tp8 kop.TapPlan, destroySelf bool) bool {
	att := strings.Contains(vc, "AttachedBy")
	for _, c := range f.Clauses {
		ok := c.Owner == whoAny && c.CmpStat == cmpNone && c.Special == 0
		switch {
		case ok && c.Ctrl == whoYou && c.Flags == 0 && !att:
		case ok && c.Ctrl == whoAny && c.Flags == fEquippedBy && (!att || tp8 == kop.TapPlanDefer || destroySelf):
		default:
			return false
		}
	}
	return true
}

// tapsDestroySelf mirrors gorge's planner recognizer (pay
// PaymentPlanTapDestroySelfTrigger) over a `Mode$ Taps` watcher: the trigger's
// keys are the closed world's (Mode, ValidCard, Execute, TriggerZones naming
// Battlefield and TriggerDescription) and its body is exactly one Destroy of
// the tapped permanent itself (Brink of Disaster's "when enchanted permanent
// becomes tapped, destroy it" on the AttachedBy line; Cryoshatter's on
// EquippedBy). The trigger resolves after the payment that caused it
// (CR 405.6c/603.2) and its only effect is to destroy the permanent whose tap
// already produced its mana (CR 605.4a), so it touches no payment resource and
// the tapped source stays an ordinary planner unit on both engines
// (kop.TapPlanOrdinary) instead of a deferred one.
func tapsDestroySelf(t cards.Trigger) bool {
	for _, k := range slices.Sorted(maps.Keys(t.Params)) {
		switch k {
		case "Mode", "ValidCard", "Execute", "TriggerDescription":
		case "TriggerZones":
			if strings.TrimSpace(t.Params[k]) != "Battlefield" {
				return false
			}
		default:
			return false
		}
	}
	body := t.Effect
	if body == nil || body.API != "Destroy" || strings.TrimSpace(body.ParamStr(cards.PKSubAbility)) != "" {
		return false
	}
	if d := body.ParamStr(cards.PKDefined); strings.Contains(d, " & ") {
		return false
	} else if txt := strings.TrimSpace(d); txt != "TriggeredCardLKICopy" && txt != "TriggeredCard" {
		return false
	}
	for _, k := range slices.Sorted(maps.Keys(body.Params)) {
		switch k {
		case "API", "Defined", "SpellDescription", "StackDescription":
		default:
			return false
		}
	}
	return true
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
