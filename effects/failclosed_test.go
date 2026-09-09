package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestUnimplementedPredicateFailsClosed is the fail-closed leaf: a predicate
// this build does NOT implement still matches NOTHING and is still reported by
// UnknownPredicates -- it never silently becomes an always-true predicate that
// widens a filter. The families chosen are deliberately the biggest remaining
// ones from the census that this seat left unknown (they need per-effect
// Remembered/Imprinted tracking this build does not carry), so a future seat
// inherits an honest, measured boundary rather than a guessed one.
//
// "IsRemembered" and "ExiledWithSource" are the two largest. A card that uses
// e.g. `Card.IsRemembered` (Return of the Wildspeaker-style "you may play this
// card from exile" clauses, the exile-until-leaves shapes) must fail closed:
// the predicate matches nothing, so those clauses stay inert rather than
// firing against every card.
func TestUnimplementedPredicateFailsClosed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := corpusObject(t, reg, g, "Grizzly Bears")

	// The predicates a later seat still owes, largest first. Each must match
	// nothing -- never become an always-true predicate.
	for _, spec := range []string{
		"Card.IsRemembered",
		"Card.ExiledWithSource",
		"Creature.wasDealtDamageThisTurn",
		"Permanent.IsImprinted",
		"Creature.ChosenCard",
		"Creature.nonChosenCard",
		"Creature.HasCounters", // negative: this one IS implemented, so it breaks the loop below
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
	for _, want := range []string{"IsRemembered", "ExiledWithSource", "wasDealtDamageThisTurn", "IsImprinted", "ChosenCard", "nonChosenCard"} {
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
