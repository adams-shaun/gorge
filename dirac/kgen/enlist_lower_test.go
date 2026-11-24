package kgen

import (
	"testing"
)

// enlistCarriers print one bare K:Enlist (CR 702.154a) among lines kgen
// already lowers; each lowers to CardDef.Enlist, the declare-attackers
// election (dirac/kw_enlist.go). Aradesh, the Founder (enlistedThisCombat)
// and the three Mode$ Enlisted trigger carriers (Benalish Knight-Counselor,
// Goblin Morale Sergeant, Guardian of New Benalia) refuse on those lines and
// stay out of the list.
var enlistCarriers = []string{
	"Argivian Cavalier", "Balduvian Berserker", "Barkweave Crusher",
	"Benalish Faithbonder", "Coalition Skyknight", "Coalition Warbrute",
	"Effie, Fast Learner", "Hexbane Tortoise", "Keldon Flamesage",
	"Linebreaker Baloth", "Yavimaya Steelcrusher",
}

func TestEnlistCarriersLower(t *testing.T) {
	for _, name := range enlistCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
		if !c.Enlist {
			t.Errorf("%s: CardDef.Enlist not set", name)
		}
	}
}
