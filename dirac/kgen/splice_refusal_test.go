package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Splice onto Arcane (CR 702.47) stays refused in dirac, by keyword name.
//
// gorge has no kw:Splice handler either (engine/rules/crcoverage, CR 612.10
// not-impl): it plays these cards as their bare spell, so there is no gorge
// behaviour for dirac to match decision by decision. Lowering the keyword to
// a no-op would claim support without lockstep evidence, and modelling it
// (a cast-time choose-any-subset-of-hand reveal, the spliced cards' extra
// costs and the composed spell text with its own targets) needs a new
// decision and new spell state in both engines. Until gorge leads, the class
// is refused: this pins that every Splice card in the corpus keeps the
// refusal, so a later keyword table edit cannot drop it silently.
func TestSpliceCardsStayRefusedByKeyword(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{
		"Kodama's Might", "Desperate Ritual", "Glacial Ray", "Torrent of Stone",
		"Blessed Breath", "Goryo's Vengeance",
	}
	for _, name := range names {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		found := false
		for _, r := range lc.Refusals {
			if r.Reason == `keyword "Splice"` {
				found = true
			}
		}
		if !found {
			t.Errorf("%s refusals = %v, want one to be keyword \"Splice\"", name, lc.Refusals)
		}
	}
}
