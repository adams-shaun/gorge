package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestUnimplementedPredicateFailsClosed is the fail-closed leaf: a predicate
// this build does NOT implement still matches NOTHING and is still reported by
// UnknownPredicates -- it never silently becomes an always-true predicate that
// widens a filter. The families chosen are the remaining pc1 stragglers this
// build left unknown; the row's named object/game-context families
// (ExiledWithSource, wasDealtDamageThisTurn, IsImprinted, NotDefinedTargeted,
// DefenderCtrl, Opponent) were implemented by task pc1ctx and are asserted in
// contextword_test.go instead. A card that uses e.g. `Card.DefinedTargeted`
// must fail closed: the predicate matches nothing, so those clauses stay
// inert rather than firing against every card.
func TestUnimplementedPredicateFailsClosed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := corpusObject(t, reg, g, "Grizzly Bears")

	// The predicates a later seat still owes. Each must match nothing --
	// never become an always-true predicate.
	for _, spec := range []string{
		// the positive twin of NotDefinedTargeted (the corpus carries no
		// `DefinedTargeted` line, so it is not a regression surface today,
		// but it must not silently widen a filter if one appears)
		"Card.DefinedTargeted",
		// the by-source refinement of wasDealtDamageThisTurn
		"Creature.wasDealtDamageThisTurnBySource",
		// negative: this one IS implemented, so it breaks the loop below
		"Creature.HasCounters",
	} {
		// Every unimplemented spec must match nothing.
		if spec == "Creature.HasCounters" {
			continue // implemented; asserted in game_predicate_test.go
		}
		if MatchesObjectCtx(g, spec, bear, SpecContext{You: 0}) {
			t.Errorf("%q must fail closed (the predicate is unimplemented), but it matched", spec)
		}
	}

	// And UnknownPredicates keeps reporting each unimplemented one.
	for _, want := range []string{"DefinedTargeted", "wasDealtDamageThisTurnBySource"} {
		found := false
		for _, u := range UnknownPredicates("Card." + want) {
			if u == want {
				found = true
			}
		}
		if !found {
			t.Errorf("UnknownPredicates(Card.%s) = %v, want it to report %q", want, UnknownPredicates("Card."+want), want)
		}
	}
}
