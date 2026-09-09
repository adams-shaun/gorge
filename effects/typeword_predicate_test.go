package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestTypeWordPredicateMatchesItsType is the effects leaf for the positive
// counterpart of the generic non<X> negation: a predicate word that is a
// type/supertype/subtype word in `predicateTypeWords` (the corpus Types
// vocabulary) evaluates as hasType, and the two colour-count tests
// Colorless / MultiColor read off ColorsOf. All pinned on real corpus cards.
//
//   - Land.Basic matches a Forest (a Basic Land) and not a Wasteland (a
//     Land with no Basic type word); Land.nonBasic is unchanged.
//   - Colorless matches a Devoid creature and an artifact with no colour,
//     and not a Grizzly Bears; MultiColor matches a gold card and not a
//     mono-coloured one.
//   - An unknown <X> that is not a type word still matches nothing and is
//     still reported by UnknownPredicates (fail closed).
func TestTypeWordPredicateMatchesItsType(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	forest := corpusObject(t, reg, g, "Forest")         // Types: Basic Land Forest
	waste := corpusObject(t, reg, g, "Wasteland")       // Types: Land
	mimic := corpusObject(t, reg, g, "Eldrazi Mimic")   // {2}, Creature Eldrazi, Devoid
	expmap := corpusObject(t, reg, g, "Expedition Map") // {1}, Artifact
	bear := corpusObject(t, reg, g, "Grizzly Bears")    // {1}{G}, Creature Bear
	gold := corpusObject(t, reg, g, "Terminate")        // {B}{R}, Instant

	// Leaf 1: Land.Basic matches a basic land and not a non-basic one;
	// Land.nonBasic is unchanged.
	if !MatchesObjectCtx(g, "Land.Basic", forest, SpecContext{You: 0}) {
		t.Errorf("Land.Basic must match a Forest (a Basic Land)")
	}
	if MatchesObjectCtx(g, "Land.Basic", waste, SpecContext{You: 0}) {
		t.Errorf("Land.Basic must not match a Wasteland (no Basic type word)")
	}
	if !MatchesObjectCtx(g, "Land.nonBasic", waste, SpecContext{You: 0}) {
		t.Errorf("Land.nonBasic must still match a Wasteland")
	}
	if MatchesObjectCtx(g, "Land.nonBasic", forest, SpecContext{You: 0}) {
		t.Errorf("Land.nonBasic must still not match a Forest")
	}

	// Leaf 2, Colorless: a Devoid creature (no colour at all, CR 702.114) and
	// a colourless artifact are Colorless; a green Bear is not.
	if !MatchesObjectCtx(g, "Creature.Colorless", mimic, SpecContext{You: 0}) {
		t.Errorf("Creature.Colorless must match a Devoid creature (Eldrazi Mimic)")
	}
	if !MatchesObjectCtx(g, "Artifact.Colorless", expmap, SpecContext{You: 0}) {
		t.Errorf("Artifact.Colorless must match a colourless artifact (Expedition Map)")
	}
	if MatchesObjectCtx(g, "Creature.Colorless", bear, SpecContext{You: 0}) {
		t.Errorf("Creature.Colorless must not match a green Grizzly Bears")
	}

	// Leaf 2, MultiColor: a gold card is MultiColor; a mono-coloured one is not.
	if !MatchesObjectCtx(g, "Card.MultiColor", gold, SpecContext{You: 0}) {
		t.Errorf("Card.MultiColor must match a gold card (Terminate, {B}{R})")
	}
	if MatchesObjectCtx(g, "Card.MultiColor", bear, SpecContext{You: 0}) {
		t.Errorf("Card.MultiColor must not match a mono-coloured Grizzly Bears")
	}

	// Leaf 4: an unknown <X> that is not a type word fails closed -- it
	// matches nothing -- and UnknownPredicates keeps reporting it, so the
	// census and the matcher cannot drift apart.
	if MatchesObjectCtx(g, "Creature.Frobnicate", bear, SpecContext{You: 0}) {
		t.Errorf("Creature.Frobnicate (unknown X) must match nothing")
	}
	if MatchesObjectCtx(g, "Creature.Frobnicate", gold, SpecContext{You: 0}) {
		t.Errorf("Creature.Frobnicate (unknown X) must match nothing")
	}
	if un := UnknownPredicates("Creature.Frobnicate"); len(un) != 1 || un[0] != "Frobnicate" {
		t.Errorf("UnknownPredicates(Creature.Frobnicate) = %v, want [Frobnicate]", un)
	}

	// The matcher and UnknownPredicates must agree on the now-recognised words.
	for _, spec := range []string{"Land.Basic", "Creature.Colorless", "Artifact.Colorless", "Card.MultiColor"} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty", spec, un)
		}
	}
}
