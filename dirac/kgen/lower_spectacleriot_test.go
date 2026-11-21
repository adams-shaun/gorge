package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kw:Spectacle (CR 702.137a), the `spectacle` filter word and kw:Riot (CR
// 702.136a): the printed carriers lower with zero refusals, each carrying
// the CardDef field the kernel reads (dirac/kw_spectacle.go, kw_riot.go); a
// GRANTED Riot (gorge reads the printed keyword only) stays refused.
func TestSpectacleRiotLower(t *testing.T) {
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
		"Blade Juggler", "Body Count", "Dead Revels", "Drill Bit", "Hackrobat", "Light Up the Stage",
		"Rafter Demon", "Rix Maadi Reveler", "Skewer the Critics", "Spawn of Mayhem", "Spikewheel Acrobat",
	} {
		lc := lower(name)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
		}
		if lc.Spectacle == nil {
			t.Errorf("%s lowered without a spectacle cost", name)
		}
	}
	for _, name := range []string{
		"Arcbound Slasher", "Burning-Tree Vandal", "Clamor Shaman", "Frenzied Arynx", "Ghor-Clan Wrecker",
		"Gruul Beastmaster", "Gruul Spellbreaker", "Rampaging Rendhorn", "Skarrgan Hellkite", "Wrecking Beast", "Zhur-Taa Goblin",
	} {
		lc := lower(name)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
		}
		if !lc.Riot {
			t.Errorf("%s lowered without CardDef.Riot", name)
		}
	}
	// the granted forms: gorge poses no riot ask for a granted keyword
	for _, name := range []string{"Rhythm of the Wild", "Uncivil Unrest", "Spider-Punk"} {
		lc := lower(name)
		refused := false
		for _, r := range lc.Refusals {
			refused = refused || strings.Contains(r.Reason, "Riot")
		}
		if !refused {
			t.Errorf("%s refusals = %v, want the granted Riot refused", name, lc.Refusals)
		}
	}
}
