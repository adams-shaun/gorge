package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// IsPresent$ counts with PresentZone$ Hand or Library lower to the kernel's
// hand / library card count; each card below has no other blocker.
func TestPresentZoneHandLibraryLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{
		"Battle of Wits", "Bloodhall Priest", "Brink of Madness", "Ebony Owl Netsuke",
		"Forceful Cultivator", "Hollowborn Barghest", "Imaginary Pet", "Ivory Crane Netsuke",
		"Kefnet the Mindful", "Library of Alexandria", "Magmatic Scorchwing", "Magus of the Library",
		"Misers' Cage", "Paupers' Cage", "Resonating Lute", "Scalding Tongs", "Thumbscrews",
		"Triskaidekaphile",
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
		}
	}
}
