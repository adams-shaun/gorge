package kgen

import "testing"

// scavengeCarriers print K:Scavenge (CR 702.97a) on a fixed-power creature;
// each lowers from gorge's kwScavenge expansion, a sorcery-speed graveyard
// AB$ PutCounter whose count is the exiled card's power (Exiled$CardPower:
// the card's own printed power, the one card the cost exiles).
var scavengeCarriers = []string{
	"Bannerhide Krushok", "Deadbridge Goliath", "Dreg Mangler", "Drudge Beetle", "Golgari Decoy",
	"Korozda Monitor", "Sewer Shambler", "Slitherhead", "Sluiceway Scorpion", "Terrus Wurm",
	"Thrashing Mossdog", "Zanikev Locust",
}

func TestScavengeCarriersLower(t *testing.T) {
	for _, name := range scavengeCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// A scavenger whose power is a characteristic-defining ability (the exiled
// card's power is read where it lies, not printed) stays refused.
func TestScavengeStarPowerRefuses(t *testing.T) {
	for _, name := range []string{"Boneyard Mycodrax", "Dodgy Jalopy"} {
		if c := lowerNamedCard(t, name); len(c.Refusals) == 0 {
			t.Errorf("%s lowers; want a refusal for its star power", name)
		}
	}
}
