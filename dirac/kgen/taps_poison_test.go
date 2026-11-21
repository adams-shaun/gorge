package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsPoison: the poison row of the AttachedBy Taps watcher ("whenever
// enchanted artifact becomes tapped, its controller gets a poison counter",
// Relic Putrescence) is a post-payment consequence gorge's planner vets (pay
// PaymentPlanTapPoisonTrigger), so the enchanted source's tap stays an
// ordinary payment unit (kop.TapPlanOrdinary), like the destroy-self row's;
// a body the planner cannot vet (a draw, a compound or non-player referent,
// an extra body key) keeps the tap deferred, and a damage body to the tapped
// permanent's controller still refuses the lowering.
func TestTapsPoison(t *testing.T) {
	const line = "T:Mode$ Taps | ValidCard$ Card.AttachedBy | TriggerZones$ Battlefield | Execute$ TrigX | TriggerDescription$ x"
	for _, tc := range []struct {
		name, sv string
		want     kop.TapPlan
		refuse   bool
	}{
		{"poison the enchanted permanent's controller", "SVar:TrigX:DB$ Poison | Defined$ TriggeredCardController | Num$ 1", kop.TapPlanOrdinary, false},
		{"poison yourself", "SVar:TrigX:DB$ Poison | Defined$ You | Num$ 1", kop.TapPlanOrdinary, false},
		{"poison an opponent", "SVar:TrigX:DB$ Poison | Defined$ Opponent | Num$ 2", kop.TapPlanOrdinary, false},
		{"an unread amount refuses", "SVar:TrigX:DB$ Poison | Defined$ TriggeredCardController | Num$ Count$X", 0, true},
		{"a draw body defers", "SVar:TrigX:DB$ Draw | NumCards$ 1 | Defined$ TriggeredCardController", kop.TapPlanDefer, false},
		{"damage to the tapped permanent's controller refuses", "SVar:TrigX:DB$ DealDamage | Defined$ TriggeredCardController | NumDmg$ 2", 0, true},
		// The kernel's Poison lowering refuses the shapes whose body it cannot
		// lower at all (each opponent, a card referent, a compound one, an
		// extra key): the card stays out of the kernel's pools, the same
		// closed-world boundary the body lowering had before the row.
		{"poison each opponent refuses", "SVar:TrigX:DB$ Poison | Defined$ EachOpponent | Num$ 1", 0, true},
		{"a card referent refuses", "SVar:TrigX:DB$ Poison | Defined$ TriggeredCardLKICopy | Num$ 1", 0, true},
		{"a compound referent refuses", "SVar:TrigX:DB$ Poison | Defined$ You & TriggeredCardController | Num$ 1", 0, true},
		{"an extra body key refuses", "SVar:TrigX:DB$ Poison | Defined$ TriggeredCardController | Num$ 1 | RememberLKI$ True", 0, true},
	} {
		why, ab, _ := ktqBuildSV(t, line, tc.sv)
		if got := why != ""; got != tc.refuse {
			t.Errorf("%s: refusal %q, want refused=%v", tc.name, why, tc.refuse)
			continue
		}
		if tc.refuse {
			continue
		}
		if ab.Trig.TapPlan != tc.want {
			t.Errorf("%s: TapPlan %d, want %d", tc.name, ab.Trig.TapPlan, tc.want)
		}
	}
	// the corpus shape: Relic Putrescence's own watcher lowers ordinary
	for _, c := range lowerNamed(t, []string{"Relic Putrescence"}) {
		if len(c.Refusals) > 0 {
			t.Fatalf("%s refused: %v", c.Name, c.Refusals)
		}
		found := false
		for _, a := range c.Abilities {
			tr := a.Trigger
			if tr == nil || tr.Event != "Taps" {
				continue
			}
			found = true
			if tr.TapPlan != uint8(kop.TapPlanOrdinary) {
				t.Errorf("%s: TapPlan %d, want Ordinary", c.Name, tr.TapPlan)
			}
		}
		if !found {
			t.Fatalf("%s: no Taps trigger", c.Name)
		}
	}
}
