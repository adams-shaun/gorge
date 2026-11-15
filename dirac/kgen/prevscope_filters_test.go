package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestScopedPreventionFiltersLower: the this-turn scoped prevention Effects
// whose ValidSource$ / ValidTarget$ is a card filter (the Fog family) lower
// to Ext PreventDamageScope with the filters on the instruction.
func TestScopedPreventionFiltersLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{
		"Al-abara's Carpet", "Arachnogenesis", "Blinding Fog", "Chameleon Blur",
		"Commencement of Festivities", "Defend the Hearth", "Endure", "Ethersworn Shieldmage",
		"Fog of War", "Forfend", "Frontline Strategist", "Galadhrim Ambush", "Harmless Assault",
		"Hindervines", "Inspire Awe", "Luminesce", "Obscuring Haze", "Pack Leader", "Radiant Kavu",
		"Repel the Abominable", "Scarecrow", "Snag", "Tanglesap", "Thwart the Enemy", "Vine Snare",
	}
	for _, name := range names {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
			continue
		}
		scoped := 0
		for _, a := range lc.Abilities {
			for _, in := range a.Prog {
				if in.Op == "PreventDamageScope" {
					scoped++
				}
			}
		}
		if scoped == 0 {
			t.Errorf("%s lowers without a PreventDamageScope instruction", name)
		}
	}
}
