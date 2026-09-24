package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestAdventureCardPredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	adventure := corpusObject(t, reg, g, "Bonecrusher Giant")
	ordinary := corpusObject(t, reg, g, "Grizzly Bears")
	if adventure.Card.AlternateMode != "Adventure" || len(adventure.Card.Faces) != 2 {
		t.Fatalf("Bonecrusher Giant lacks valid Adventure shape: mode=%q faces=%d", adventure.Card.AlternateMode, len(adventure.Card.Faces))
	}
	if ordinary.Card.AlternateMode == "Adventure" {
		t.Fatal("Grizzly Bears unexpectedly has Adventure mode")
	}
	if !MatchesObjectCtx(g, "Creature.AdventureCard", adventure, SpecContext{You: 0}) {
		t.Fatal("Creature.AdventureCard did not match Bonecrusher Giant")
	}
	if MatchesObjectCtx(g, "Creature.AdventureCard", ordinary, SpecContext{You: 0}) {
		t.Fatal("Creature.AdventureCard matched Grizzly Bears")
	}
	if !MatchesObjectCtx(g, "Creature.!AdventureCard", ordinary, SpecContext{You: 0}) {
		t.Fatal("Creature.!AdventureCard did not match Grizzly Bears")
	}
	if MatchesObjectCtx(g, "Creature.!AdventureCard", adventure, SpecContext{You: 0}) {
		t.Fatal("Creature.!AdventureCard matched Bonecrusher Giant")
	}
	if un := UnknownPredicates("Creature.AdventureCard"); len(un) != 0 {
		t.Fatalf("UnknownPredicates reports recognized predicate unknown: %v", un)
	}
}
