package kgen

import "testing"

// vanishingCarriers print K:Vanishing (CR 702.63a) and nothing else kgen
// refuses; each lowers from gorge's kwVanishing expansion: the enters-with
// time counters replacement, the upkeep removal and the last-counter
// sacrifice (Mode$ CounterRemoved, CounterType$ TIME, NewCounterAmount$ 0).
var vanishingCarriers = []string{
	"Aven Riftwatcher", "Calciderm", "Deadly Grub", "Deep Forest Hermit",
	"Dreamtide Whale", "Keldon Marauders", "Lavacore Elemental", "Lost Auramancers",
	"Ravaging Riftwurm", "Soultether Golem", "Tidewalker", "Waning Wurm",
}

func TestVanishingCarriersLower(t *testing.T) {
	for _, name := range vanishingCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}
