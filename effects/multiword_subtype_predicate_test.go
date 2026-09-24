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
	tardis := corpusObject(t, reg, g, "TARDIS")
	lord := corpusObject(t, reg, g, "TARDIS")
	cardCopy := *lord.Card
	faceCopy := *lord.Card.Faces[0]
	faceCopy.Types = []string{"Creature", "Time", "Lord"}
	cardCopy.Faces = []*cards.Face{&faceCopy}
	lord.Card = &cardCopy
	if lord.Zone != state.ZBattlefield || tardis.Zone != state.ZBattlefield {
		t.Fatal("test objects must be on the battlefield")
	}
	spec := "Card.Time Lord+YouCtrl"
	if un := UnknownPredicates(spec); len(un) != 0 {
		t.Fatalf("UnknownPredicates(%q) = %v, want none", spec, un)
	}
	if !MatchesObjectCtx(g, spec, lord, SpecContext{You: 0}) {
		t.Fatal("Card.Time Lord+YouCtrl must match a controlled Time Lord")
	}
	if MatchesObjectCtx(g, spec, tardis, SpecContext{You: 0}) {
		t.Fatal("Card.Time Lord+YouCtrl must reject an object missing the Time Lord subtype")
	}
	unsupported := "Card.Time Zorb+YouCtrl"
	if MatchesObjectCtx(g, unsupported, lord, SpecContext{You: 0}) {
		t.Fatal("unsupported multiword predicate must fail closed")
	}
	if un := UnknownPredicates(unsupported); len(un) != 1 || un[0] != "Time Zorb" {
		t.Fatalf("UnknownPredicates(%q) = %v, want [Time Zorb]", unsupported, un)
	}
	if !MatchesObjectCtx(g, "Creature.Time Lord+YouCtrl", lord, SpecContext{You: 0}) {
		t.Fatal("TARDIS-shaped IsPresent filter must match controlled Time Lord")
	}
}
