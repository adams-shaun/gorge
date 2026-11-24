package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChooserCounterCostLowering: gorge's chooser counter cost
// AddCounter<N/M1M1/Creature.YouCtrl[/text]> (cost/token_match.go) lowers to
// the same Blight cost as Blight<N> — the spell and activated carriers with
// zero refusals, kop.CostBlight, N 1 and a non-zero creature filter — while
// the unless spelling (Thelon's and Tourach's Chant) and a zero count refuse.
func TestChooserCounterCostLowering(t *testing.T) {
	for _, name := range []string{"Hatchet Bully", "Wandering Mage", "Lethal Sting", "Scarscale Ritual"} {
		all, bs := keclBuild(t, []string{name})
		i := keclIndex(t, all, name)
		if r := lowerRefusals(all[i]); len(r) != 0 {
			t.Fatalf("%s refused: %v", name, r)
		}
		var c kop.Cost
		for _, a := range bs[i].def.Abilities {
			if a.Cost.Flags&kop.CostBlight != 0 {
				c = a.Cost
			}
		}
		if c.Flags&kop.CostBlight == 0 {
			t.Fatalf("%s: no ability carries CostBlight", name)
		}
		if c.N != 1 {
			t.Errorf("%s: Blight N = %d, want 1", name, c.N)
		}
		if c.Filter == 0 {
			t.Errorf("%s: Blight has no creature-you-control filter", name)
		}
	}

	// The chants' same spelling inside an UnlessCost$ refuses as before.
	for _, name := range []string{"Thelon's Chant", "Tourach's Chant"} {
		all, _ := keclBuild(t, []string{name})
		i := keclIndex(t, all, name)
		if got := keclReason(lowerRefusals(all[i])); !strings.Contains(got, "AddCounter target \"Creature.YouCtrl\"") {
			t.Errorf("%s: refusals %q, want the unless AddCounter reason", name, got)
		}
	}

	// A zero count refuses, mirroring Blight<0>.
	body := "Name:Counter Probe\nManaCost:1 B\nTypes:Creature Human\nPT:1/1\n" +
		"A:AB$ GainLife | Cost$ AddCounter<0/M1M1/Creature.YouCtrl> | LifeAmount$ 1\n"
	if got := keclReason(keclSynth(t, "Counter Probe", body)); !strings.Contains(got, "counter cost") {
		t.Errorf("AddCounter<0/M1M1/Creature.YouCtrl>: refusals %q, want the counter-cost reason", got)
	}
}
