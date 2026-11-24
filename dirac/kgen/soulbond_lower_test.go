package kgen

import "testing"

// soulbondCarriers print K:Soulbond (CR 702.95a); each lowers from gorge's
// kwSoulbond expansion (the two ETB pair triggers, api Pair) and a static or
// trigger over the pair (Creature.PairedWith, Creature.Self+Paired).
var soulbondCarriers = []string{
	"Deadeye Navigator", "Diregraf Escort", "Druid's Familiar", "Elgaud Shieldmate",
	"Galvanic Alchemist", "Geist Trappers", "Hanweir Lancer", "Lightning Mauler",
	"Nearheath Pilgrim", "Nightshade Peddler", "Pathbreaker Wurm", "Silverblade Paladin",
	"Spectral Gateguards", "Stern Mentor", "Stonewright", "Tandem Lookout",
	"Trusted Forcemage", "Wingcrafter", "Wolfir Silverheart",
}

func TestSoulbondLowers(t *testing.T) {
	for _, name := range soulbondCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}
