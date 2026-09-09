package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// corpusObject builds a battlefield object carrying a REAL corpus card's face
// (Merfolk of the Pearl Trident is a blue Merfolk creature; Grizzly Bears is a
// green Bear), so the generic non<X> negation is exercised against the type
// and colour vocabulary the engine actually carries -- never a synthetic face.
// All objects share one game, matching the shape MatchesObjectCtx is called
// with in the engine.
func corpusObject(t *testing.T, reg *cards.Registry, g *state.Game, name string) *state.Object {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus has no %q", name)
	}
	o := g.AddObject(c, 0)
	o.Zone = state.ZBattlefield
	return o
}

// TestNonPredicateGenericNegation is the effects leaf for the generic non<X>
// predicate negation:
//
//   - Creature.nonMerfolk matches a non-Merfolk creature and not a Merfolk
//     (a subtype word, previously a hand-written-only gap -- only
//     nonLand/nonCreature/nonBasic/nonBlack existed, so Creature.nonMerfolk
//     matched NOTHING and Walk the Plank could not be cast);
//   - Creature.nonBlue matches by colour (via ColorsOf, the way nonBlack does
//     it), not by type;
//   - Creature.nonFrobnicate (an unknown <X>) matches NOTHING -- it must fail
//     closed, never become an always-true !hasType that silently widens.
//
// All three pins use real corpus cards, never synthetic faces.
func TestNonPredicateGenericNegation(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	merfolk := corpusObject(t, reg, g, "Merfolk of the Pearl Trident") // U, Merfolk
	bear := corpusObject(t, reg, g, "Grizzly Bears")                   // G, Bear

	// Subtype negation: nonMerfolk holds for the Bear, not for the Merfolk.
	if !MatchesObjectCtx(g, "Creature.nonMerfolk", bear, SpecContext{You: 0}) {
		t.Errorf("Creature.nonMerfolk must match a non-Merfolk creature (Grizzly Bears)")
	}
	if MatchesObjectCtx(g, "Creature.nonMerfolk", merfolk, SpecContext{You: 0}) {
		t.Errorf("Creature.nonMerfolk must not match a Merfolk")
	}

	// Colour negation: nonBlue holds for the green Bear, not for the blue
	// Merfolk. That it is a COLOUR test is the point: the Merfolk has the
	// Merfolk type but is blue, so a type-based negation would wrongly pass it.
	if !MatchesObjectCtx(g, "Creature.nonBlue", bear, SpecContext{You: 0}) {
		t.Errorf("Creature.nonBlue must match a non-blue creature (Grizzly Bears)")
	}
	if MatchesObjectCtx(g, "Creature.nonBlue", merfolk, SpecContext{You: 0}) {
		t.Errorf("Creature.nonBlue must not match a blue Merfolk")
	}

	// Unknown <X> fails closed: it matches nothing, never everything.
	if MatchesObjectCtx(g, "Creature.nonFrobnicate", bear, SpecContext{You: 0}) {
		t.Errorf("Creature.nonFrobnicate (unknown X) must match nothing")
	}
	if MatchesObjectCtx(g, "Creature.nonFrobnicate", merfolk, SpecContext{You: 0}) {
		t.Errorf("Creature.nonFrobnicate (unknown X) must match nothing")
	}

	// UnknownPredicates must agree: a non<X> the matcher can now evaluate is no
	// longer reported unknown, while an unknown <X> still is.
	for _, spec := range []string{"Creature.nonMerfolk", "Creature.nonBlue"} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty", spec, un)
		}
	}
	if un := UnknownPredicates("Creature.nonFrobnicate"); len(un) != 1 || un[0] != "nonFrobnicate" {
		t.Errorf("UnknownPredicates(Creature.nonFrobnicate) = %v, want [nonFrobnicate]", un)
	}
}
