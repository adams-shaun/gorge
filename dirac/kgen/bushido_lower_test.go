package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A printed K:Bushido line is gorge's expansion (cards.kwBushido): a Blocks
// and an AttackerBlocked trigger on Card.Self, each a DB$ Pump of +N/+N. kgen
// lowers those two lines, so the keyword is in expandedKeywords and a face
// carrying it has no refusal. (Fumiko, Sensei Golden-Tail and Takeno read the
// keyword in a static, an Animate and a count, which stay refused.)
func TestBushidoCardsLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Jade Avenger", "Samurai of the Pale Curtain", "Hand of Honor", "Kitsune Blademaster", "Konda, Lord of Eiganjo"} {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Fatalf("%s refusals = %v, want none", name, lc.Refusals)
		}
		blocks, blocked := 0, 0
		for _, ab := range lc.Abilities {
			if ab.Trigger != nil && ab.Trigger.Event == "Blocks" {
				blocks++
			}
			if ab.Trigger != nil && ab.Trigger.Event == "AttackerBlocked" {
				blocked++
			}
		}
		if blocks != 1 || blocked != 1 {
			t.Errorf("%s lowers %d Blocks and %d AttackerBlocked triggers, want one of each", name, blocks, blocked)
		}
	}
}
