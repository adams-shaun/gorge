package kgen

import "testing"

// TestSnowCostLowers: an activated ability's {S} cost (CR 107.4h) lowers to
// kop.ManaCost.Snow; a printed spell cost with {S} keeps refusing.
func TestSnowCostLowers(t *testing.T) {
	for _, tc := range []struct {
		name string
		want int
	}{
		{"Boreal Centaur", 1}, {"Icehide Troll", 2}, {"Thermopod", 1}, {"Frost Raptor", 1}, {"Phyrexian Ironfoot", 1},
	} {
		lc := lowerCorpusCard(t, tc.name)
		if r := probeRefusals(lc); r != "" {
			t.Errorf("%s refused: %s", tc.name, r)
			continue
		}
		best := 0
		for _, a := range lc.Abilities {
			best = max(best, a.Cost.Mana.Snow)
		}
		if best < tc.want {
			t.Errorf("%s: activated ability Cost.Mana.Snow = %d, want >= %d", tc.name, best, tc.want)
		}
	}
	if r := probeRefusals(lowerCorpusCard(t, "Icehide Golem")); r == "" {
		t.Errorf("Icehide Golem (a printed {S} spell cost) lowered; it must stay refused")
	}
}
