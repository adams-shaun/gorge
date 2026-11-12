package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestIsImprintedMatchesTheReturnedCard: Tainted Pact's check
// Count$ValidHand Card.IsImprinted asks whether the card the Dig imprinted
// was put into hand; the ordinary Imprinted association must still name it
// there. A card that left exile for the graveyard is not a match.
func TestIsImprintedMatchesTheReturnedCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	sourceID := corpusObject(t, reg, g, "Knowledge Pool").ID
	cardID := corpusObject(t, reg, g, "Grizzly Bears").ID
	card := g.Obj(cardID)
	source := g.Obj(sourceID)
	source.Imprinted = []state.ObjID{cardID}

	card.Zone = state.ZHand
	g.SetZone(state.ZHand, card.Owner, []state.ObjID{cardID})
	sc := SpecContext{You: 0, Source: sourceID, HandCount: true}
	if !MatchesObjectCtx(g, "Card.IsImprinted", card, sc) {
		t.Fatal("Card.IsImprinted must match the returned card in a Count$ValidHand scan")
	}
	// An ordinary live read keeps the CR 607.2a expiry.
	if MatchesObjectCtx(g, "Card.IsImprinted", card, SpecContext{You: 0, Source: sourceID}) {
		t.Error("Card.IsImprinted must not match a hand card outside a Count$ValidHand scan")
	}

	card.Zone = state.ZGraveyard
	g.SetZone(state.ZGraveyard, card.Owner, []state.ObjID{cardID})
	if MatchesObjectCtx(g, "Card.IsImprinted", card, sc) {
		t.Error("Card.IsImprinted must not match after the card left exile for a graveyard")
	}
}
