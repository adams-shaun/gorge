package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestIsImprintedStopsMatchingAfterOrdinaryImprintLeavesExile(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	sourceID := corpusObject(t, reg, g, "Knowledge Pool").ID
	imprintedID := corpusObject(t, reg, g, "Grizzly Bears").ID
	bystanderID := corpusObject(t, reg, g, "Grizzly Bears").ID
	source := g.Obj(sourceID)
	linked := g.Obj(imprintedID)
	bystander := g.Obj(bystanderID)
	source.Imprinted = []state.ObjID{imprintedID}
	linked.Zone = state.ZExile
	g.SetZone(state.ZExile, linked.Owner, []state.ObjID{imprintedID})
	if linked.Zone != state.ZExile || linked.ID == bystander.ID {
		t.Fatalf("precondition failed: linked candidate must be distinct and in exile (linked=%+v, bystander=%d)", linked, bystander.ID)
	}
	sc := SpecContext{You: 0, Source: sourceID}
	if !MatchesObjectCtx(g, "Card.IsImprinted", linked, sc) {
		t.Fatal("Card.IsImprinted must match the linked card while it is in exile")
	}
	if MatchesObjectCtx(g, "Card.IsImprinted", bystander, sc) {
		t.Fatal("Card.IsImprinted matched an unlinked bystander")
	}

	// CR 607.2a: the ordinary imprint link no longer identifies the card once
	// it leaves exile. The stale ID intentionally remains in source.Imprinted.
	linked.Zone = state.ZGraveyard
	g.SetZone(state.ZGraveyard, linked.Owner, []state.ObjID{imprintedID})
	if linked.Zone == state.ZExile {
		t.Fatal("precondition failed: linked card must have left exile")
	}
	if MatchesObjectCtx(g, "Card.IsImprinted", linked, sc) {
		t.Error("Card.IsImprinted must stop matching an ordinary linked card after it leaves exile")
	}
}
