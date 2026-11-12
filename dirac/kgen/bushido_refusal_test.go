package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Both ticket cards carry a printed K:Bushido line. gorge now expands it,
// but dirac has no Bushido lowering and must keep refusing the cards by
// name with the keyword reason: a silent keyword drop would claim support
// without lockstep evidence (and move buildAcceptancePin).
func TestBushidoCardsStayRefusedByKeyword(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Jade Avenger", "Samurai of the Pale Curtain"} {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 1 || lc.Refusals[0].Reason != `keyword "Bushido"` {
			t.Fatalf("%s refusals = %v, want exactly [keyword \"Bushido\"]", name, lc.Refusals)
		}
	}
}
