package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestHasANonBasicLandTypePredicate is the effects leaf for Forge's
// Card.hasANonBasicLandType, the predicate the corpus spells as
// `Land.hasANonBasicLandType` (Wonderscape Sage's ConditionPresent gate).
//
// The predicate is CR 205.3i: a LAND that has at least one land type
// OUTSIDE the five basic land types (Plains / Island / Swamp / Mountain /
// Forest). It is deliberately NOT a simple negation of hasABasicLandType and
// NOT the `Land.nonBasic` supertype test:
//
//   - a Desert (Land Desert) and a Gate (Land Gate) both have a nonbasic land
//     type and match;
//   - a Forest / Island has only a basic land type and does not match;
//   - a Wasteland (Land, no land subtype) is a NONBASIC land but has no
//     nonbasic LAND TYPE, so it does not match -- the one case that
//     separates this predicate from `Land.nonBasic`;
//   - a non-land creature or artifact does not match.
//
// The classifier and the matcher share wordPredicate, so UnknownPredicates
// reports the bare token recognised (the pc1 structural rule): the two
// cannot disagree about whether `Land.hasANonBasicLandType` is implemented.
func TestHasANonBasicLandTypePredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	desert := corpusObject(t, reg, g, "Desert")             // Land Desert
	gate := corpusObject(t, reg, g, "Boros Guildgate")      // Land Gate
	forest := corpusObject(t, reg, g, "Forest")             // Basic Land Forest
	island := corpusObject(t, reg, g, "Island")             // Basic Land Island
	waste := corpusObject(t, reg, g, "Wasteland")           // Land (no subtype)
	grizzly := corpusObject(t, reg, g, "Grizzly Bears")     // Creature Bear
	relic := corpusObject(t, reg, g, "Relic of Progenitus") // Artifact

	sc := SpecContext{You: 0}

	// Positive: the corpus carrier's own `Land.<pred>` spelling matches both
	// qualifier spellings, for both nonbasic land types.
	for _, spec := range []string{"Land.hasANonBasicLandType", "Card.hasANonBasicLandType"} {
		for _, o := range []*state.Object{desert, gate} {
			if !MatchesObjectCtx(g, spec, o, sc) {
				t.Errorf("%s must match %s (a nonbasic land type)", spec, o.Face().Name)
			}
		}
	}

	// Precondition the Wasteland negative depends on: it IS a land, and it IS
	// a nonbasic land, yet it carries no nonbasic land TYPE. Assert both so a
	// corpus regression that changed its types would fail loudly instead of
	// making the negative below vacuous.
	if !hasTypeCtx(waste, "Land", sc) {
		t.Fatal("precondition: Wasteland must be a Land")
	}
	if !MatchesObjectCtx(g, "Land.nonBasic", waste, sc) {
		t.Fatal("precondition: Wasteland must be a nonbasic land (Land.nonBasic)")
	}
	if MatchesObjectCtx(g, "Land.hasANonBasicLandType", waste, sc) {
		t.Error("Land.hasANonBasicLandType must NOT match Wasteland (a nonbasic land with no nonbasic land TYPE)")
	}
	// Basic-typed lands are the negative half of the predicate and the
	// preconditions that they really carry their basic type.
	for _, o := range []*state.Object{forest, island} {
		if !hasTypeCtx(o, o.Face().Name, sc) {
			t.Fatalf("precondition: %s must carry its own basic land type", o.Face().Name)
		}
		if MatchesObjectCtx(g, "Land.hasANonBasicLandType", o, sc) {
			t.Errorf("Land.hasANonBasicLandType must not match %s (only a basic land type)", o.Face().Name)
		}
	}
	if MatchesObjectCtx(g, "Land.hasANonBasicLandType", grizzly, sc) {
		t.Error("Land.hasANonBasicLandType must not match a non-land creature")
	}
	if MatchesObjectCtx(g, "Land.hasANonBasicLandType", relic, sc) {
		t.Error("Land.hasANonBasicLandType must not match a non-land artifact")
	}

	// Census agreement: the matcher and UnknownPredicates share wordPredicate,
	// so the bare token is recognised (no unknown predicates reported).
	if un := UnknownPredicates("Land.hasANonBasicLandType"); len(un) != 0 {
		t.Errorf("UnknownPredicates(Land.hasANonBasicLandType) = %v, want empty", un)
	}
	// A genuinely unknown predicate alongside it is still reported, so the
	// assertion above is not passing because the census reports nothing at
	// all for this spec shape.
	if un := UnknownPredicates("Land.hasANonBasicLandType+totallyNotAPredicate"); len(un) != 1 {
		t.Errorf("UnknownPredicates of a mixed spec = %v, want exactly the unknown token", un)
	}
}
