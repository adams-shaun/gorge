package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Rampage N (CR 702.23a) lowers: gorge's kwRampage expands it to an
// AttackerBlocked self trigger whose Pump reads the blockers of the
// triggered attacker (the Johtull Wurm count), so every printed carrier
// lowers with no refusal.
func TestRampageLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{
		"Aerathi Berserker", "Balduvian War-Makers", "Chromium", "Craw Giant",
		"Eldest Dragon Highlander", "Frost Giant", "Gorilla Berserkers", "Horrible Hordes",
		"Hunding Gjornersen", "Marhault Elsdragon", "Teeka's Dragon", "Wolverine Pack",
	} {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		if _, has := card.Faces[0].KeywordParam("Rampage"); !has {
			t.Fatalf("%s carries no Rampage keyword: the test would pass vacuously", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
		}
	}
}
