package pay

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects/params"
)

// PaymentPlanTapDamageTrigger vets the foreign tap-damage trigger row
// (Hawkeye's Bow's "whenever equipped creature becomes tapped, it deals 1
// damage to each opponent"): closed-world over the trigger's parameters and a
// single literal DealDamage body naming a player; PaymentPlanTapDamageHits
// says whether that damage can reach the payer, the only payment resource it
// could consume. Anything else stays unknown and its source deferred.
func TestTapDamageTriggerShape(t *testing.T) {
	t.Parallel()
	bow := &cards.SA{API: "DealDamage", Params: map[string]string{
		"Defined": "Opponent", "NumDmg": "1", "DamageSource": "TriggeredCardLKICopy"}}
	trig := func(params map[string]string, body *cards.SA) cards.Trigger {
		return cards.Trigger{Mode: "Taps", Params: params, Effect: body}
	}
	bowTrig := trig(map[string]string{"ValidCard": "Creature.EquippedBy", "Execute": "TrigDamage",
		"TriggerDescription": "Whenever equipped creature becomes tapped, it deals 1 damage to each opponent."}, bow)

	n, ref, ok := PaymentPlanTapDamageTrigger(nil, bowTrig)
	if !ok || n != 1 || ref.Text != "Opponent" {
		t.Fatalf("Bow: got %d, %v, %v", n, ref, ok)
	}

	// The payer controls the Bow: the damage cannot reach it, so the tapped
	// source stays an ordinary unit; the trigger fires on its own at the tap.
	if PaymentPlanTapDamageHits(ref, 0, 0, 1) {
		t.Fatal("Bow's opponent damage reached its own controller as payer")
	}
	// The payer is the Bow's opponent: the damage is its life, disclosed.
	if !PaymentPlanTapDamageHits(ref, 0, 1, 0) {
		t.Fatal("the Bow controller's opponent is hit and must disclose")
	}
	youRef := params.Ref{Kind: params.RefYou, Text: "You"}
	if !PaymentPlanTapDamageHits(youRef, 1, 1, 0) || PaymentPlanTapDamageHits(youRef, 1, 0, 0) {
		t.Fatal("Defined$ You reaches only the trigger's controller")
	}
	tccRef := params.Ref{Text: "TriggeredCardController"}
	if !PaymentPlanTapDamageHits(tccRef, 1, 1, 1) || PaymentPlanTapDamageHits(tccRef, 1, 1, 0) {
		t.Fatal("Defined$ TriggeredCardController reaches only the tapped permanent's controller")
	}

	// Shapes the planner cannot vet stay unknown.
	bad := map[string]cards.Trigger{
		"mode":        {Mode: "TapsForMana", Params: map[string]string{"ValidCard": "Creature.EquippedBy"}, Effect: bow},
		"firsttime":   {Mode: "Taps", Params: map[string]string{"ValidCard": "Creature.YouCtrl", "FirstTime": "True"}, Effect: bow},
		"zones":       {Mode: "Taps", Params: map[string]string{"ValidCard": "Creature.EquippedBy", "TriggerZones": "Graveyard"}, Effect: bow},
		"extra param": {Mode: "Taps", Params: map[string]string{"ValidCard": "Creature.EquippedBy", "OptionalDecider": "You"}, Effect: bow},
		"card dmg":    {Mode: "Taps", Params: map[string]string{"ValidCard": "Creature.EquippedBy"}, Effect: &cards.SA{API: "DealDamage", Params: map[string]string{"Defined": "TriggeredCardLKICopy", "NumDmg": "1"}}},
		"sub":         {Mode: "Taps", Params: map[string]string{"ValidCard": "Creature.EquippedBy"}, Effect: &cards.SA{API: "DealDamage", Params: map[string]string{"Defined": "Opponent", "NumDmg": "1", "SubAbility": "More"}}},
		"not literal": {Mode: "Taps", Params: map[string]string{"ValidCard": "Creature.EquippedBy"}, Effect: &cards.SA{API: "DealDamage", Params: map[string]string{"Defined": "Opponent", "NumDmg": "X"}}},
	}
	for name, tr := range bad {
		if _, _, ok := PaymentPlanTapDamageTrigger(nil, tr); ok {
			t.Errorf("%s: classified as known", name)
		}
	}
	// Defined$ You is the self-damage row's shape and a known player body:
	// the observer discloses it through PaymentPlanTapDamageHits.
	you := &cards.SA{API: "DealDamage", Params: map[string]string{"Defined": "You", "NumDmg": "2"}}
	if n, _, ok := PaymentPlanTapDamageTrigger(nil, trig(map[string]string{"ValidCard": "Card.AttachedBy"}, you)); !ok || n != 2 {
		t.Fatalf("You body: got %d, %v", n, ok)
	}
}
