package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// umbraCycle is the printed umbra Auras: K:Umbra armor lowers to
// CardDef.UmbraArmor and nothing else refuses.
var umbraCycle = []string{
	"Bear Umbra", "Boar Umbra", "Crab Umbra", "Drake Umbra", "Eel Umbra",
	"Eland Umbra", "Felidar Umbra", "Hyena Umbra", "Indrik Umbra", "Lion Umbra",
	"Mammoth Umbra", "Snake Umbra", "Spider Umbra", "Treefolk Umbra",
}

func TestUmbraArmorLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range umbraCycle {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
		}
		if !lc.UmbraArmor {
			t.Errorf("%s: UmbraArmor not set", name)
		}
	}
}
