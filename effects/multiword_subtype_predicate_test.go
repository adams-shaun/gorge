package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestMultiWordSubtypePredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	// Use the real TARDIS face for its intervening-if shape, but keep the
	// Time Lord under test synthetic so the test pins the shared type matcher.
	tardis := corpusObject(t, reg, g, "TARDIS")
	if tardis.Zone != state.ZBattlefield {
		t.Fatal("TARDIS must be on the battlefield")
	}
	both := *tardis
	both.ID++
	face := *tardis.Card.Faces[0]
	face.Types = []string{"Artifact", "Time", "Lord"}
	card := *tardis.Card
	card.Faces = []*cards.Face{&face}
	both.Card = &card
	both.Zone = state.ZBattlefield
	missingLord := both
	faceMissingLord := face
	faceMissingLord.Types = []string{"Artifact", "Time"}
	cardMissingLord := card
	cardMissingLord.Faces = []*cards.Face{&faceMissingLord}
	missingLord.Card = &cardMissingLord
	missingTime := both
	faceMissingTime := face
	faceMissingTime.Types = []string{"Artifact", "Lord"}
	cardMissingTime := card
	cardMissingTime.Faces = []*cards.Face{&faceMissingTime}
	missingTime.Card = &cardMissingTime
	ctx := SpecContext{You: 0}
	if !MatchesObjectCtx(g, "Card.Time Lord+YouCtrl", &both, ctx) {
		t.Fatal("Card.Time Lord+YouCtrl must match when both type words are present")
	}
	if MatchesObjectCtx(g, "Card.Time Lord+YouCtrl", &missingLord, ctx) || MatchesObjectCtx(g, "Card.Time Lord+YouCtrl", &missingTime, ctx) {
		t.Fatal("multiword subtype must fail when either constituent type word is absent")
	}
	if got := UnknownPredicates("Card.Time Lord+YouCtrl"); len(got) != 0 {
		t.Fatalf("supported predicate reported unknown: %v", got)
	}
	if MatchesObjectCtx(g, "Card.Time Lordish Unknown+YouCtrl", &both, ctx) {
		t.Fatal("unknown multiword predicate must fail closed")
	}
	if got := UnknownPredicates("Card.Time Lordish Unknown"); len(got) != 1 {
		t.Fatalf("unknown multiword census = %v, want one unknown token", got)
	}
	// TARDIS's exact IsPresent filter spelling is accepted by the same matcher.
	if !MatchesObjectCtx(g, "Card.Time Lord+YouCtrl", &both, ctx) {
		t.Fatal("TARDIS-shaped IsPresent predicate must match a Time Lord")
	}
}
