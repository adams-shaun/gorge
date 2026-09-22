package effects

// The compound player-spec grammar (`Player.X+Y`, `!IsRemembered`) and the
// `Card.EffectSource` object predicate, pinned at the filter-grammar level.
// The end-to-end corpus pins live in
// rules/chooseplayer_subability_test.go (Territorial Hellkite): the compound
// `Choices$ Player.Opponent+!IsRemembered` pool and the Effect-delivered
// `ValidCreature$ Card.EffectSource` requirement.

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestPlayerSpecCompoundConjunctionAndNegation pins the `+` conjunction and
// leading `!` negation against a source object's event-backed Chosen and
// Remembered player sets. A clause with no `+` is unchanged; an absent source
// fails the bare property clauses closed.
func TestPlayerSpecCompoundConjunctionAndNegation(t *testing.T) {
	g := state.NewGame([]string{"a", "b", "c"})
	src := qualifierObject(t, g, 0, "Name:Source\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	// Seat 0 is the source's controller (the "you" perspective), seats 1 and
	// 2 are opponents. Seat 1 is both chosen and remembered; seat 2 is
	// neither.
	src.Chosen = []state.Target{{Player: 1, IsPlayer: true}}
	src.Remembered = []state.Target{{Player: 1, IsPlayer: true}}

	cases := []struct {
		spec string
		seat state.PlayerID
		want bool
	}{
		{"Player.Opponent+!IsRemembered", 1, false}, // remembered opponent excluded
		{"Player.Opponent+!IsRemembered", 2, true},  // un-remembered opponent admitted
		{"Player.Opponent+!IsRemembered", 0, false}, // the controller is not an opponent
		{"Player.Opponent+IsRemembered", 1, true},
		{"Player.Opponent+IsRemembered", 2, false},
		{"Player.Opponent+Chosen", 1, true},
		{"Player.Opponent+!Chosen", 2, true},
		{"Player.Opponent", 1, true}, // the single-clause form is unchanged
		{"!IsRemembered", 2, true},
		{"!IsRemembered", 1, false},
	}
	for _, c := range cases {
		if got := MatchesPlayerSpecFrom(g, c.spec, c.seat, 0, src.ID); got != c.want {
			t.Errorf("MatchesPlayerSpecFrom(%q, seat %d) = %v, want %v", c.spec, c.seat, got, c.want)
		}
	}

	// Precondition: with no source bound the bare property clause fails
	// closed, exactly as it did before the compound grammar existed.
	if MatchesPlayerSpec(g, "Player.Opponent+!IsRemembered", 2, 0) {
		t.Fatal("an unbound source admitted an un-remembered opponent")
	}
}

// TestEffectSourcePredicate pins `Card.EffectSource`: it matches the object
// named by SpecContext.Source and nothing else, and fails closed when no
// source is bound.
func TestEffectSourcePredicate(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	src := qualifierObject(t, g, 0, "Name:Self Beast\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	other := qualifierObject(t, g, 0, "Name:Other Beast\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")

	sc := SpecContext{You: 0, Source: src.ID}
	if !MatchesSpecCtx(g, "Card.EffectSource", src.ID, sc) {
		t.Fatal("Card.EffectSource did not match the bound source")
	}
	if MatchesSpecCtx(g, "Card.EffectSource", other.ID, sc) {
		t.Fatal("Card.EffectSource matched a different object")
	}
	if MatchesSpecCtx(g, "Card.EffectSource", src.ID, SpecContext{You: 0}) {
		t.Fatal("Card.EffectSource matched with no source bound")
	}
	// The conjunction spelling the corpus also uses still works.
	if !MatchesSpecCtx(g, "Creature.EffectSource", src.ID, sc) {
		t.Fatal("Creature.EffectSource did not match a bound creature source")
	}
}
