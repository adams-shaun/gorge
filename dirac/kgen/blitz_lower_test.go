package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// blitzCarriers print one K:Blitz:<mana cost> (CR 702.152a) among lines kgen
// already lowers; each lowers to CardDef.HasBlitz with its cost, the hand's
// blitzed cast (dirac/kw_blitz.go). The cards whose other lines refuse stay
// out of the list: Effluence Devourer (a Perpetual animate), Riveteers
// Provocateur (withoutBlitz), Wave of Rats and Ziatora's Envoy; the two
// non-mana blitz costs (Sabin, Master Monk's Discard<1/Card>, Tenacious
// Underdog's PayLife<2>) refuse with their graveyard-cast statics. Star
// Athlete lowers here but is refused by name (dirac/support/refused): its
// untargeted up-to-one Mode$ Attacks trigger diverges from gorge with no
// blitz involved.
var blitzCarriers = []string{
	"Caldaia Guardian", "Caldaia Strongarm", "Girder Goons", "Jaxis, the Troublemaker",
	"Mayhem Patrol", "Mezzio Mugger", "Night Clubber", "Plasma Jockey",
	"Pugnacious Pugilist", "Riveteers Decoy", "Riveteers Requisitioner", "Star Athlete",
	"Workshop Warchief",
}

func TestBlitzCarriersLower(t *testing.T) {
	for _, name := range blitzCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
		if !c.Blitz || c.BlitzCost == (kop.ManaCost{}) {
			t.Errorf("%s: Blitz %v cost %+v, want a lowered blitz cost", name, c.Blitz, c.BlitzCost)
		}
	}
}

// TestBlitzNonManaCostsRefuse: a blitz cost with a Discard or PayLife part
// is no mana alternative cost; the kernel prices mana only.
func TestBlitzNonManaCostsRefuse(t *testing.T) {
	for _, name := range []string{"Sabin, Master Monk", "Tenacious Underdog"} {
		c := lowerNamedCard(t, name)
		if c.Blitz {
			t.Errorf("%s lowered a non-mana blitz cost", name)
		}
		if len(c.Refusals) == 0 {
			t.Errorf("%s: no refusal", name)
		}
	}
}
