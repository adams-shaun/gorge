package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLowerTapsformanaSelfZhurTaa lowers the corpus card Zhur-Taa Druid: its
// "Whenever you tap CARDNAME for mana, it deals 1 damage to each opponent"
// line is a self TapsForMana trigger. gorge's planner defers the source
// (pay.PaymentPlanTapDamageTrigger accepts Mode$ Taps only), so the lowered
// line must stay kop.TapPlanDefer, not Ordinary; the self clause becomes the
// builder's kop.TfSelf and Activator$ You the line's WhoseYou.
func TestLowerTapsformanaSelfZhurTaa(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Zhur-Taa Druid"})
	var card *lCard
	for _, c := range lw.cards {
		if c.Name == "Zhur-Taa Druid" {
			card = c
		}
	}
	if card == nil {
		t.Fatal("Zhur-Taa Druid not lowered")
	}
	if len(card.Refusals) != 0 {
		t.Fatalf("Zhur-Taa Druid refused: %v", card.Refusals)
	}
	var trig *lTrigger
	for _, a := range card.Abilities {
		if a.Kind == akTriggered && a.Trigger != nil && a.Trigger.Event == "Taps" && a.Trigger.TapsForMana {
			trig = a.Trigger
		}
	}
	if trig == nil {
		t.Fatal("precondition: Zhur-Taa Druid has no lowered TapsForMana line")
	}
	if trig.Whose != whoYou {
		t.Errorf("Whose %d, want whoYou (%d)", trig.Whose, whoYou)
	}
	if trig.TapPlan != uint8(kop.TapPlanDefer) {
		t.Errorf("TapPlan %d, want Defer", trig.TapPlan)
	}
	if trig.F == nil || len(trig.F.Clauses) != 1 || trig.F.Clauses[0] != (lClause{Flags: fSelf}) {
		t.Errorf("ValidCard filter %+v, want the bare Card.Self clause", trig.F)
	}
	// The built CardDef is what the kernel reads: the self clause became
	// kop.TfSelf off a zero filter.
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	self := false
	for i, c := range all {
		if c.Name != "Zhur-Taa Druid" {
			continue
		}
		for j := range bs[i].def.Abilities {
			a := &bs[i].def.Abilities[j]
			if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigTaps || a.Trig.Flags5&kop.Tf5TapsForMana == 0 {
				continue
			}
			self = true
			if a.Trig.Flags&kop.TfSelf == 0 {
				t.Error("built TapsForMana line lacks kop.TfSelf")
			}
			if a.Trig.Filter != 0 {
				t.Errorf("built self line filter %d, want 0", a.Trig.Filter)
			}
			if a.Trig.Whose != kop.WhoseYou {
				t.Errorf("built Whose %d, want WhoseYou", a.Trig.Whose)
			}
			if a.Trig.TapPlan != kop.TapPlanDefer {
				t.Errorf("built TapPlan %d, want Defer", a.Trig.TapPlan)
			}
		}
	}
	if !self {
		t.Fatal("built Zhur-Taa Druid has no self TapsForMana line")
	}
}
