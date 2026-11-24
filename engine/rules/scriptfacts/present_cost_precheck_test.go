package scriptfacts

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestPresentIsOwnCostPrecheck pins the rule presentGateApplies (package rules)
// and kgen triggerIsPresentConds share: an IsPresent$ that only mirrors the
// trigger's own SubCounter cost, with no printed intervening-if, is not a gate
// (Sun Droplet, CR 603.2); the same shape printed "..., if CARDNAME has ..."
// (Ingenious Prodigy, CR 603.4) and any non-mirroring clause keep the gate.
func TestPresentIsOwnCostPrecheck(t *testing.T) {
	const mirror = "T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ Player | IsPresent$ Card.Self+counters_GE1_CHARGE | Execute$ TrigGain | TriggerZones$ Battlefield | OptionalDecider$ You | TriggerDescription$ At the beginning of each upkeep, you may remove a charge counter from CARDNAME. If you do, you gain 1 life.\n"
	for _, c := range []struct {
		name, trig, svar string
		want             bool
	}{
		{"cost mirror", mirror, "SVar:TrigGain:AB$ GainLife | Cost$ SubCounter<1/CHARGE> | LifeAmount$ 1 | Defined$ You\n", true},
		{"printed intervening-if", "T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | IsPresent$ Card.Self+counters_GE1_CHARGE | Execute$ TrigGain | TriggerZones$ Battlefield | TriggerDescription$ At the beginning of your upkeep, if CARDNAME has a charge counter on it, you may remove a charge counter from it. If you do, you gain 1 life.\n",
			"SVar:TrigGain:AB$ GainLife | Cost$ SubCounter<1/CHARGE> | LifeAmount$ 1 | Defined$ You\n", false},
		{"other counter kind", mirror, "SVar:TrigGain:AB$ GainLife | Cost$ SubCounter<1/KI> | LifeAmount$ 1 | Defined$ You\n", false},
		{"other amount", mirror, "SVar:TrigGain:AB$ GainLife | Cost$ SubCounter<2/CHARGE> | LifeAmount$ 1 | Defined$ You\n", false},
		{"present rider", "T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ Player | IsPresent$ Card.Self+counters_GE1_CHARGE | PresentPlayer$ Opponent | Execute$ TrigGain | TriggerZones$ Battlefield | TriggerDescription$ At the beginning of each upkeep, you may remove a charge counter from CARDNAME. If you do, you gain 1 life.\n",
			"SVar:TrigGain:AB$ GainLife | Cost$ SubCounter<1/CHARGE> | LifeAmount$ 1 | Defined$ You\n", false},
		{"no cost", mirror, "SVar:TrigGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\n", false},
	} {
		src := "Name:Test Droplet\nManaCost:2\nTypes:Artifact\n" + c.trig + c.svar + "Oracle:x\n"
		card, diags := cards.ParseBytes("test_droplet.txt", []byte(src))
		if card == nil || len(card.Faces) == 0 || len(card.Faces[0].Triggers) != 1 {
			t.Fatalf("%s: parse failed: %v", c.name, diags)
		}
		card.Link()
		tr := card.Faces[0].Triggers[0]
		if tr.Effect == nil {
			t.Fatalf("%s: precondition: Execute$ was not linked", c.name)
		}
		if got := PresentIsOwnCostPrecheck(tr); got != c.want {
			t.Errorf("%s: PresentIsOwnCostPrecheck = %v, want %v", c.name, got, c.want)
		}
	}
}
