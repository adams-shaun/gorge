package kgen

import "testing"

// championCarriers print K:Champion (CR 702.72a); each lowers from gorge's
// kwChampion expansion: an enters trigger exiling another [type] you control
// until the champion leaves, sacrificing the champion when nothing was exiled.
var championCarriers = []string{
	"Boggart Mob", "Changeling Berserker", "Changeling Hero", "Changeling Titan", "Lightning Crafter",
	"Nova Chaser", "Supreme Exemplar", "Thoughtweft Trio", "Unstoppable Ash", "Wanderwine Prophets",
	"Wren's Run Packmaster",
}

func TestChampionLowers(t *testing.T) {
	for _, name := range championCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}
