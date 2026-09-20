package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestIsNotChosenType is the effects leaf for the inverse of the positive
// ChosenType filter:
//
//   - Creature.IsNotChosenType matches a creature that is not of the
//     resolving source's chosen type (Kindred Dominance's "destroy all
//     creatures that aren't of the chosen type", Crippling Fear's -3/-3,
//     Callous Oppressor's target, Raise the Palisade's return);
//   - it rejects a creature that IS of the chosen type;
//   - with no type chosen on the source it matches NOTHING -- the
//     fail-closed guard, because "not of the chosen type" is meaningless
//     before a type is chosen and an always-true reading would silently
//     widen the filter;
//   - the census agrees with the matcher: UnknownPredicates reports nothing
//     for Creature.IsNotChosenType (and Creature.ChosenType stays clean), the
//     matcher/census-agreement contract the positive path already keeps.
//
// Real corpus faces throughout, matching the shape MatchesObjectCtx is called
// with in the engine (SpecContext.Source points at the resolving source).
func TestIsNotChosenType(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	goblin := corpusObject(t, reg, g, "Goblin Piker") // R, Goblin Warrior
	bear := corpusObject(t, reg, g, "Grizzly Bears")  // G, Bear

	// The resolving source is a real corpus carrier (Kindred Dominance); only
	// its ChosenType stamp matters to the predicate.
	src := corpusObject(t, reg, g, "Kindred Dominance")
	sc := SpecContext{You: 0, Source: src.ID}

	// With a type chosen, the inverse holds for the non-matching creature and
	// fails for the matching one.
	src.ChosenType = "Goblin"
	if !MatchesObjectCtx(g, "Creature.IsNotChosenType", bear, sc) {
		t.Errorf("Creature.IsNotChosenType must match a non-Goblin creature (Grizzly Bears) with Goblin chosen")
	}
	if MatchesObjectCtx(g, "Creature.IsNotChosenType", goblin, sc) {
		t.Errorf("Creature.IsNotChosenType must not match a Goblin with Goblin chosen")
	}

	// Fail-closed guard: no chosen type means the spec matches nothing, never
	// everything.
	src.ChosenType = ""
	if MatchesObjectCtx(g, "Creature.IsNotChosenType", bear, sc) {
		t.Errorf("Creature.IsNotChosenType must fail closed (match nothing) with no chosen type")
	}
	if MatchesObjectCtx(g, "Creature.IsNotChosenType", goblin, sc) {
		t.Errorf("Creature.IsNotChosenType must fail closed (match nothing) with no chosen type")
	}

	// The census shares the matcher's classifier: both spellings recognised.
	if got := UnknownPredicates("Creature.IsNotChosenType"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Creature.IsNotChosenType) = %v, want empty (the census must agree with the matcher)", got)
	}
	if got := UnknownPredicates("Creature.ChosenType"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Creature.ChosenType) = %v, want empty (the positive path must stay recognised)", got)
	}
}
