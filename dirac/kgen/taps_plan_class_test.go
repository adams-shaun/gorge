package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsPlanClass: a lowered non-Self Taps watcher carries the payment-
// planner class gorge's tier gives the source its tap touches
// (pay.PaymentPlanAbilityTier): vetted literal player damage is Pain when its
// referent can name the payer (You, TriggeredCardController) and Ordinary when
// it cannot (Opponent, EachOpponent); every other body defers the source.
func TestTapsPlanClass(t *testing.T) {
	const line = "T:Mode$ Taps | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ TrigX | TriggerDescription$ x"
	for _, tc := range []struct {
		name string
		sv   string
		want kop.TapPlan
		pain uint8
	}{
		{"gain life", "SVar:TrigX:DB$ GainLife | LifeAmount$ 1", kop.TapPlanDefer, 0},
		{"damage to you", "SVar:TrigX:DB$ DealDamage | Defined$ You | NumDmg$ 2", kop.TapPlanPain, 2},
		{"damage to the tapped permanent's controller", "SVar:TrigX:DB$ DealDamage | Defined$ TriggeredCardController | NumDmg$ 2", kop.TapPlanPain, 2},
		{"damage to an opponent", "SVar:TrigX:DB$ DealDamage | Defined$ Opponent | NumDmg$ 2", kop.TapPlanOrdinary, 0},
		{"damage to the tapper", "SVar:TrigX:DB$ DealDamage | Defined$ TriggeredActivator | NumDmg$ 2", kop.TapPlanDefer, 0},
	} {
		why, ab, _ := ktqBuildSV(t, line, tc.sv)
		if why != "" {
			t.Errorf("%s: refused: %s", tc.name, why)
			continue
		}
		if ab.Trig.Event != kop.TrigTaps {
			t.Errorf("%s: event %v, want TrigTaps", tc.name, ab.Trig.Event)
			continue
		}
		if ab.Trig.TapPlan != tc.want || ab.Trig.TapPainN != tc.pain {
			t.Errorf("%s: class %d pain %d, want %d pain %d", tc.name, ab.Trig.TapPlan, ab.Trig.TapPainN, tc.want, tc.pain)
		}
	}
	// the corpus shapes: Judge of Currents defers, Hawkeye's Bow is ordinary,
	// Captain America's FirstTime$ untap body defers the source like Judge of Currents
	for _, c := range lowerNamed(t, []string{"Judge of Currents", "Hawkeye's Bow", "Captain America, Living Legend"}) {
		if len(c.Refusals) > 0 {
			t.Errorf("%s: refused: %v", c.Name, c.Refusals)
			continue
		}
		want := map[string]uint8{"Judge of Currents": uint8(kop.TapPlanDefer), "Hawkeye's Bow": uint8(kop.TapPlanOrdinary), "Captain America, Living Legend": uint8(kop.TapPlanDefer)}[c.Name]
		found := false
		for _, a := range c.Abilities {
			if a.Trigger != nil && a.Trigger.Event == "Taps" {
				found = true
				if a.Trigger.TapPlan != want {
					t.Errorf("%s: lowered class %d, want %d", c.Name, a.Trigger.TapPlan, want)
				}
			}
		}
		if !found {
			t.Errorf("%s: no Taps trigger lowered", c.Name)
		}
	}
}
