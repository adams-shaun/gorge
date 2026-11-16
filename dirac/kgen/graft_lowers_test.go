package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Graft N (CR 702.58a) lowers from gorge's expansion: the entering
// counters replacement and the may-move trigger on another creature.
func TestGraftLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{
		"Aquastrand Spider", "Cytoplast Manipulator", "Cytoplast Root-Kin", "Cytospawn Shambler",
		"Helium Squirter", "Llanowar Reborn", "Novijen Sages", "Plaxcaster Frogling",
		"Simic Initiate", "Sporeback Troll", "Vigean Graftmage", "Vigean Hydropon",
	} {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
		}
	}
}
