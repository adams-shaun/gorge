package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSelfTapsPainOnManaCard: a card's own "whenever CARDNAME becomes tapped,
// it deals N damage to you" beside a plain {T} mana ability (City of Brass)
// lowers as a Pain-class Self Taps line, as gorge's planner reads it
// (pay.PaymentPlanSelfDamageTrigger: the source's own last-resort consequence).
// Any other body, a damage line beyond the vetted parameters, or a mana
// ability beyond a plain {T} still refuses by name.
func TestSelfTapsPainOnManaCard(t *testing.T) {
	const head = "Name:Brass Probe\nManaCost:no cost\nTypes:Land\n"
	const mana = "A:AB$ Mana | Cost$ T | Produced$ Any | SpellDescription$ Probe any.\n"
	const trig = "T:Mode$ Taps | ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ TrigProbe | TriggerDescription$ Probe pain.\n"
	const body = "SVar:TrigProbe:DB$ DealDamage | Defined$ You | NumDmg$ 1\n"
	def, rs := kwardpowBuild(t, "Brass Probe", head+mana+trig+body)
	if len(rs) != 0 {
		t.Fatalf("City of Brass's shape refused: %v", rs)
	}
	found := false
	for _, a := range def.Abilities {
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps {
			found = true
			if a.Trig.Flags&kop.TfSelf == 0 || a.Trig.TapPlan != kop.TapPlanPain || a.Trig.TapPainN != 1 {
				t.Errorf("Taps line flags %v class %d pain %d, want Self, Pain, 1", a.Trig.Flags, a.Trig.TapPlan, a.Trig.TapPainN)
			}
		}
	}
	if !found {
		t.Fatal("no Taps line lowered")
	}
	for _, c := range []struct{ name, src string }{
		{"a gain-life body", head + mana + trig + "SVar:TrigProbe:DB$ GainLife | LifeAmount$ 1\n"},
		{"damage to an opponent", head + mana + trig + "SVar:TrigProbe:DB$ DealDamage | Defined$ Opponent | NumDmg$ 1\n"},
		{"a mana ability with a fee", head + "A:AB$ Mana | Cost$ 1 T | Produced$ Any | SpellDescription$ x.\n" + trig + body},
		{"two mana", head + "A:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2 | SpellDescription$ x.\n" + trig + body},
	} {
		_, rs := kwardpowBuild(t, "Brass Probe", c.src)
		var all []string
		for _, r := range rs {
			all = append(all, r.Reason)
		}
		if !strings.Contains(strings.Join(all, "; "), "Taps trigger on a card with a mana ability") {
			t.Errorf("%s: refusals %q, want the Taps-on-a-mana-card refusal", c.name, all)
		}
	}
	// the corpus card
	for _, c := range lowerNamed(t, []string{"City of Brass"}) {
		if len(c.Refusals) > 0 {
			t.Errorf("%s: refused: %v", c.Name, c.Refusals)
		}
	}
}
