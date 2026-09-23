package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestHasABasicLandTypePredicate is the effects leaf for Forge's
// Card.hasABasicLandType, the predicate the corpus spells as
// `Land.hasABasicLandType` (Sprouting Goblin's kicked ETB, Slimefoot's
// Survey, Perilous Forays, Nervous Gardener, Boseiju, Lukamina).
//
// The predicate is CR 205.3i: a LAND that has at least one of the five
// basic land types (Plains / Island / Swamp / Mountain / Forest). It is not
// "has the Basic supertype": a Wastes is a basic land with NO basic land
// type and must NOT match, and a non-land permanent granted a land type by
// a continuous effect is excluded by the Land base the body re-checks.
//
// The classifier and the matcher share wordPredicate, so UnknownPredicates
// reports the bare token recognised (the pc1 structural rule): the two
// cannot disagree about whether `Land.hasABasicLandType` is implemented.
func TestHasABasicLandTypePredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	forest := corpusObject(t, reg, g, "Forest")             // Basic Land Forest
	waste := corpusObject(t, reg, g, "Wasteland")           // Land (no basic type)
	plains := corpusObject(t, reg, g, "Plains")             // Basic Land Plains
	grizzly := corpusObject(t, reg, g, "Grizzly Bears")     // Creature Bear
	relic := corpusObject(t, reg, g, "Relic of Progenitus") // Artifact

	sc := SpecContext{You: 0}

	// Positive: the five basic land types match, and the corpus carrier's own
	// `Land.<pred>` spelling is the shape under test.
	for _, spec := range []string{"Land.hasABasicLandType", "Card.hasABasicLandType"} {
		if !MatchesObjectCtx(g, spec, forest, sc) {
			t.Errorf("%s must match a Forest (a basic land type)", spec)
		}
		if !MatchesObjectCtx(g, spec, plains, sc) {
			t.Errorf("%s must match a Plains (a basic land type)", spec)
		}
	}
	// Negative: a Wasteland is a Land with no basic land type; a creature and
	// an artifact are not lands at all. Assert the precondition the negative
	// depends on -- Wasteland IS a land -- so a corpus regression that made
	// the fixture not a land would fail loudly instead of passing vacuously.
	if !hasTypeCtx(waste, "Land", sc) {
		t.Fatal("precondition: Wasteland must be a Land")
	}
	if MatchesObjectCtx(g, "Land.hasABasicLandType", waste, sc) {
		t.Error("Land.hasABasicLandType must not match a Wasteland (no basic land type)")
	}
	if MatchesObjectCtx(g, "Land.hasABasicLandType", grizzly, sc) {
		t.Error("Land.hasABasicLandType must not match a non-land creature")
	}
	if MatchesObjectCtx(g, "Land.hasABasicLandType", relic, sc) {
		t.Error("Land.hasABasicLandType must not match a non-land artifact")
	}

	// Census agreement: the matcher and UnknownPredicates share wordPredicate,
	// so the bare token is recognised (no unknown predicates reported).
	if un := UnknownPredicates("Land.hasABasicLandType"); len(un) != 0 {
		t.Errorf("UnknownPredicates(Land.hasABasicLandType) = %v, want empty", un)
	}
	// A genuinely unknown predicate alongside it is still reported, so the
	// assertion above is not passing because the census reports nothing at
	// all for this spec shape.
	if un := UnknownPredicates("Land.hasABasicLandType+totallyNotAPredicate"); len(un) != 1 {
		t.Errorf("UnknownPredicates of a mixed spec = %v, want exactly the unknown token", un)
	}
}
