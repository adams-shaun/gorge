package kgen

import (
	"testing"
)

// riotCarriers print one bare K:Riot (CR 702.136a) among lines kgen already
// lowers; each lowers to CardDef.Riot, the as-enters counter-or-haste election
// (kw_riot.go). Spider-Punk (a static grants riot to other Spiders) and every
// other granted-riot carrier refuse and stay out of the list; Ravager Wurm
// too (its charm's Land.hasNonManaActivatedAbility filter is unmodelled).
var riotCarriers = []string{
	"Arcbound Slasher", "Burning-Tree Vandal", "Clamor Shaman", "Frenzied Arynx",
	"Ghor-Clan Wrecker", "Gruul Beastmaster", "Gruul Spellbreaker",
	"Rampaging Rendhorn", "Skarrgan Hellkite", "Wrecking Beast", "Zhur-Taa Goblin",
}

func TestRiotCarriersLower(t *testing.T) {
	for _, name := range riotCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
		if !c.Riot {
			t.Errorf("%s: CardDef.Riot not set", name)
		}
	}
}
