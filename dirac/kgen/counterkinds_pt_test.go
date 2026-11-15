package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPTCounterKindsLower: the cards whose only blocker was a P/T counter
// kind other than +1/+1, -1/-1 and -2/-1 lower and build (CR 122.1a).
func TestPTCounterKindsLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{
		"Armor Thrull", "Balduvian Hydra", "Baron Sengir", "Brass-Talon Chimera", "Dwarven Armorer",
		"Dwarven Armory", "Ebon Praetor", "Essence Flare", "Fungus Elemental", "Greater Werewolf",
		"Iron-Heart Chimera", "Krovikan Plague", "Lead-Belly Chimera", "Lesser Werewolf", "Lightning Serpent",
		"Living Armor", "Shield Sphere", "Spirit Shackle", "Tin-Wing Chimera", "Wall of Resistance",
	} {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s lowering refusals = %v, want none", name, lc.Refusals)
			continue
		}
		// the counter kind is named when the lowered card is built
		if b := buildCard(newPoolTables(&lowered{cards: []*lCard{lc}}, nil), lc); len(b.refusals) != 0 {
			t.Errorf("%s build refusals = %v, want none", name, b.refusals)
		}
	}
}
