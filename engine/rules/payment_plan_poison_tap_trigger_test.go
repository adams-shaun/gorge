package rules

// A Taps trigger whose only effect is a poison counter on a player the
// payment cannot move (Relic Putrescence's "whenever enchanted artifact
// becomes tapped, its controller gets a poison counter") is a post-payment
// consequence, not tap interference: the mana was produced at the tap (CR
// 605.4a) and the trigger resolves after the payment that caused it (CR
// 405.6c/603.2), so the planner prices the enchanted source as an ordinary
// unit and a cast that needs it is offered, exactly as the destroy-self row
// (Brink of Disaster) is.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const pttRelic = "Name:Relic Putrescence\nManaCost:2 B\nTypes:Enchantment Aura\nK:Enchant:Artifact\nT:Mode$ Taps | ValidCard$ Card.AttachedBy | TriggerZones$ Battlefield | Execute$ TrigPoison | TriggerDescription$ Whenever enchanted artifact becomes tapped, its controller gets a poison counter.\nSVar:TrigPoison:DB$ Poison | Defined$ TriggeredCardController | Num$ 1\nOracle:x\n"

// pttEnchantedTreasure boards Tainted Field, a Swamp and three Treasures and
// attaches Relic Putrescence to the first Treasure, and returns the three
// Treasure objects (first is the enchanted one).
func pttEnchantedTreasure(t *testing.T, e *Engine) (t1, t2, t3 state.ObjID) {
	t.Helper()
	onBoard(t, e, 0, "Name:Tainted Field\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\nA:AB$ Mana | Cost$ T | Produced$ Combo W B | IsPresent$ Swamp.YouCtrl | SpellDescription$ Add {W} or {B}. Activate only if you control a Swamp.\n")
	onBoard(t, e, 0, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n")
	t1, t2, t3 = lrTreasure(t, e), lrTreasure(t, e), lrTreasure(t, e)
	relic := onBoard(t, e, 0, pttRelic)
	e.emit(events.Event{Kind: events.Attach, Obj: relic, IDs: []state.ObjID{t1}})
	return t1, t2, t3
}

// {4}{W} over five sources (two lands, three Treasures, one enchanted) needs
// the enchanted Treasure's mana: the planner prices it where it once answered
// "insufficient / source:interference:Relic Putrescence", and the witness
// names it.
func TestPaymentPlanPoisonTapTriggerPays(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 77101, lrSpell("4 W"))
	t1, _, _ := pttEnchantedTreasure(t, e)
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan == nil {
		t.Fatalf("no plan (reason %q detail %q), want the enchanted Treasure priced", got.Reason, got.Detail)
	}
	found := false
	for _, a := range got.Plan.Activations {
		if a.Source == t1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("plan %+v skips the enchanted Treasure", *got.Plan)
	}
	if err := e.ValidateCastPayment(0, paymentCast(spell), *got.Plan); err != nil {
		t.Fatalf("ValidateCastPayment(poison-tap plan): %v", err)
	}
}

// PaymentPlanTapPoisonTrigger vets the Relic Putrescence row and refuses the
// shapes the planner cannot fold: another body, a non-literal or non-positive
// amount, a compound or card referent, and a trigger outside the battlefield.
func TestPaymentPlanTapPoisonTriggerShape(t *testing.T) {
	t.Parallel()
	c := card(t, pttRelic)
	f := c.Faces[0]
	tap := f.Triggers[0]
	if !pay.PaymentPlanTapPoisonTrigger(f, tap) {
		t.Fatal("the Relic Putrescence poison row is not vetted")
	}
	// A non-poison body is not the row.
	bad := card(t, "Name:Relic Drain\nManaCost:2 B\nTypes:Enchantment Aura\nK:Enchant:Artifact\nT:Mode$ Taps | ValidCard$ Card.AttachedBy | TriggerZones$ Battlefield | Execute$ TrigDraw | TriggerDescription$ Whenever enchanted artifact becomes tapped, its controller draws a card.\nSVar:TrigDraw:DB$ Draw | Defined$ TriggeredCardController | Num$ 1\nOracle:x\n")
	if pay.PaymentPlanTapPoisonTrigger(bad.Faces[0], bad.Faces[0].Triggers[0]) {
		t.Fatal("a draw body is not the poison row")
	}
	// The amount is unread: a dynamic Num$ is still the row.
	bad = card(t, "Name:Relic Plague\nManaCost:2 B\nTypes:Enchantment Aura\nK:Enchant:Artifact\nT:Mode$ Taps | ValidCard$ Card.AttachedBy | TriggerZones$ Battlefield | Execute$ TrigPoison | TriggerDescription$ Whenever enchanted artifact becomes tapped, its controller gets a poison counter.\nSVar:TrigPoison:DB$ Poison | Defined$ TriggeredCardController | Num$ Count$X\nOracle:x\n")
	if !pay.PaymentPlanTapPoisonTrigger(bad.Faces[0], bad.Faces[0].Triggers[0]) {
		t.Fatal("a dynamic Num$ amount is still the poison row: the payment cannot move it")
	}
	// An extra body key is not the row.
	bad = card(t, "Name:Relic Rot\nManaCost:2 B\nTypes:Enchantment Aura\nK:Enchant:Artifact\nT:Mode$ Taps | ValidCard$ Card.AttachedBy | TriggerZones$ Battlefield | Execute$ TrigPoison | TriggerDescription$ Whenever enchanted artifact becomes tapped, its controller gets a poison counter.\nSVar:TrigPoison:DB$ Poison | Defined$ TriggeredCardController | RememberLKI$ True\nOracle:x\n")
	if pay.PaymentPlanTapPoisonTrigger(bad.Faces[0], bad.Faces[0].Triggers[0]) {
		t.Fatal("an extra body key is not the poison row")
	}
}
