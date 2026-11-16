package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kw:Unleash (CR 702.86): the thirteen Return-to-Ravnica Rakdos creatures
// whose printed K:Unleash is their only blocker lower with zero refusals,
// each carrying CardDef.Unleash for the kernel's as-enters ask and block gate
// (dirac/kw_unleash.go).
func TestUnleashLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lower := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}
	for _, name := range []string{
		"Bloodfray Giant", "Carnival Hellsteed", "Chaos Imps", "Dead Reveler",
		"Exava, Rakdos Blood Witch", "Gore-House Chainwalker", "Grim Roustabout",
		"Hellhole Flailer", "Rakdos Cackler", "Rakdos Drake",
		"Spawn of Rix Maadi", "Splatter Thug", "Thrill-Kill Assassin",
	} {
		lc := lower(name)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
		}
		if !lc.Unleash {
			t.Errorf("%s lowered without CardDef.Unleash", name)
		}
	}
}
