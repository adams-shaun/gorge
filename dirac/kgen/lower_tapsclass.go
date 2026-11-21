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

// tapsDeferPain is tapsPlanClass's complementary read for a body gorge's
// planner defers but whose damage to the tapper is still literal:
// `Mode$ TapsForMana` with `Defined$ TriggeredActivator` (Manabarbs),
// the player who tapped the land for mana, i.e. the payer. The planner
// defers the source (a TapsForMana body is no vetted payer-damage row,
// lower_tapsformana.go), but the tap still costs the payer the literal
// amount, so the kernel's potential census carries it in the per-tap
// lethal guard (mana_tapsdefer.go tapsWatchClass, legal.go potCensus).
// The closed world is tapsPlanClass's; any other referent is 0.
func tapsDeferPain(body *cards.SA) uint8 {
	if body == nil || body.API != "DealDamage" || strings.TrimSpace(body.ParamStr(cards.PKSubAbility)) != "" {
		return 0
	}
	for _, k := range slices.Sorted(maps.Keys(body.Params)) {
		switch k {
		case "API", "Defined", "NumDmg", "DamageSource", "SpellDescription", "StackDescription":
		default:
			return 0
		}
	}
	n, err := strconv.ParseUint(strings.TrimSpace(body.ParamStr(cards.PKNumDmg)), 10, 8)
	if err != nil || n < 1 {
		return 0
	}
	if strings.TrimSpace(body.ParamStr(cards.PKDefined)) != "TriggeredActivator" {
		return 0
	}
	return uint8(n)
}

// tapsAttachedClass is the payment-planner class of the AttachedBy Taps
// watcher: a body whose only effect is to destroy the tapped permanent itself
// (Brink of Disaster's, Cryoshatter's) or to put a poison counter on a player
// the payment cannot move (Relic Putrescence's) is a post-payment consequence
// gorge's planner vets (pay PaymentPlanTapConsequenceTrigger: the
// destroy-self and poison rows), so the source stays an ordinary unit
// (kop.TapPlanOrdinary); an unvetted body defers, as gorge's planner defers
// any tap a non-damage watcher sees; a damage body (Psychic Venom's payer
// damage) has a payment tiering the kernel does not mirror yet, so the caller
// refuses it. The bool is the vetted-consequence row found (the destroy-self
// or poison one), the flag tapsAttachedClauses vets the clause table on.
func tapsAttachedClass(t cards.Trigger) (kop.TapPlan, uint8, bool) {
	tp8, pn := tapsPlanClass(t.Effect)
	if tp8 == kop.TapPlanDefer {
		if tapsDestroySelf(t) || tapsPoison(t) {
			tp8 = kop.TapPlanOrdinary
			return tp8, pn, true
		}
	}
	return tp8, pn, false
}

// tapsAttachedClauses vets the clause table of a non-Self Taps watcher with no
// ValidPlayer$: every clause is a plain ownerless one over your own creature
// (Ctrl You, no flags, no AttachedBy) or, on the enchanted-permanent watcher
// (any Ctrl, the EquippedBy flag), carries a plan the payment planner can
// mirror — a deferring body or a vetted consequence one (destroy-self,
// poison). An unvetted AttachedBy body vetoes the lowering.
func tapsAttachedClauses(f *lFilter, vc string, tp8 kop.TapPlan, consequence bool) bool {
	att := strings.Contains(vc, "AttachedBy")
	for _, c := range f.Clauses {
		ok := c.Owner == whoAny && c.CmpStat == cmpNone && c.Special == 0
		switch {
		case ok && c.Ctrl == whoYou && c.Flags == 0 && !att:
		case ok && c.Ctrl == whoAny && c.Flags == fEquippedBy && (!att || tp8 == kop.TapPlanDefer || consequence):
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
		case cards.PKMode.String(), cards.PKValidCard.String(), cards.PKExecute.String(), cards.PKTriggerDescription.String():
		case cards.PKTriggerZones.String():
			if strings.TrimSpace(t.Params[k]) != tapsBattlefieldZone {
				return false
			}
		default:
			return false
		}
	}
	body := t.Effect
	if body == nil || body.APIKind() != cards.APIDestroy || strings.TrimSpace(body.ParamStr(cards.PKSubAbility)) != "" {
		return false
	}
	if d := body.ParamStr(cards.PKDefined); strings.Contains(d, " & ") {
		return false
	} else if txt := strings.TrimSpace(d); !slices.Contains(tapsDestroySelfRefs, txt) {
		return false
	}
	for _, k := range slices.Sorted(maps.Keys(body.Params)) {
		switch k {
		case tapsDestroySelfAPIKey, cards.PKDefined.String(), cards.PKSpellDescription.String(), cards.PKStackDescription.String():
		default:
			return false
		}
	}
	return true
}

// tapsDestroySelfRefs are the Defined$ selectors that name the tapped
// permanent itself (the LKI copy and the plain card), the closed world of
// pay.PaymentPlanTapDestroySelfTrigger.
var tapsDestroySelfRefs = []string{"TriggeredCardLKICopy", "TriggeredCard"}

// tapsPoisonAPI is the Poison effect's API name, the closed world of pay's
// paymentPlanPoisonAPI.
const tapsPoisonAPI = "Poison"

// tapsPoison mirrors gorge's planner recognizer (pay
// PaymentPlanTapPoisonTrigger) over a `Mode$ Taps` watcher: the trigger's
// keys are the closed world's (Mode, ValidCard, Execute, TriggerZones naming
// Battlefield and TriggerDescription) and its body is exactly one Poison of a
// player the payment cannot move (Relic Putrescence's "whenever enchanted
// artifact becomes tapped, its controller gets a poison counter" on the
// AttachedBy line). The trigger resolves after the payment that caused it (CR
// 405.6c/603.2) and its only effect is a poison counter on that player, so it
// touches no payment resource and the tapped source stays an ordinary planner
// unit on both engines (kop.TapPlanOrdinary). The body's Num$ is unread: the
// amount the effect applies changes nothing about the payment that triggered
// it. A trigger whose shape the planner cannot vet (another referent, a
// compound one, a sub-chain, an extra body key) is not known and the caller
// keeps the body's own class.
func tapsPoison(t cards.Trigger) bool {
	for _, k := range slices.Sorted(maps.Keys(t.Params)) {
		switch k {
		case cards.PKMode.String(), cards.PKValidCard.String(), cards.PKExecute.String(), cards.PKTriggerDescription.String():
		case cards.PKTriggerZones.String():
			if strings.TrimSpace(t.Params[k]) != tapsBattlefieldZone {
				return false
			}
		default:
			return false
		}
	}
	body := t.Effect
	if body == nil || body.API != tapsPoisonAPI || strings.TrimSpace(body.ParamStr(cards.PKSubAbility)) != "" {
		return false
	}
	d := body.ParamStr(cards.PKDefined)
	if strings.Contains(d, " & ") {
		return false
	}
	switch strings.TrimSpace(d) {
	case "You", "Opponent", "EachOpponent", "TriggeredCardController":
	default:
		return false
	}
	for _, k := range slices.Sorted(maps.Keys(body.Params)) {
		switch k {
		case tapsDestroySelfAPIKey, cards.PKDefined.String(), cards.PKNum.String(), cards.PKSpellDescription.String(), cards.PKStackDescription.String():
		default:
			return false
		}
	}
	return true
}

// tapsDestroySelfAPIKey is the dispatch key a constructed body may keep in
// Params beside SA.API; tapsBattlefieldZone is the Forge TriggerZones$ spelling
// the planner's zone check accepts.
const (
	tapsDestroySelfAPIKey = "API"
	tapsBattlefieldZone   = "Battlefield"
)

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
