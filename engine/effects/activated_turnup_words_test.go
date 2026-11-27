package effects

import (
	"testing"
)

// TestActivatedAndTurnedFaceUpThisTurnWords is the truth table for the two
// per-turn provenance filter words the residual hosted-Commander ticket
// added: activated (Cut Short's Planeswalker.activated, reading the
// events.Apply activation tally) and turnedFaceUpThisTurn (Kaust, Eyes of
// the Glade, reading the TurnFaceUp fold's mark; engine/events'
// TestTurnedFaceUpThisTurnFold pins the fold itself). The preconditions set
// the tallies/mark directly and check them, so a vacuous board cannot pass
// the table for the wrong reason.
func TestActivatedAndTurnedFaceUpThisTurnWords(t *testing.T) {
	g, id := board(t)
	walker := g.Obj(id["myWalker"])
	bear := g.Obj(id["myBear"])

	// Precondition: neither fact is present at first.
	if walker.ActivatedThisTurn != 0 || bear.ActivatedThisTurn != 0 ||
		walker.TurnedFaceUpThisTurn || bear.TurnedFaceUpThisTurn {
		t.Fatalf("precondition: a fresh board carries the facts")
	}
	if got := MatchesSpec(g, "Planeswalker.activated", walker.ID, 0); got {
		t.Fatalf("precondition: an unactivated planeswalker matched activated")
	}

	// The activation tally: only the permanent that activated matches, and
	// only its own type's spec.
	walker.ActivatedThisTurn = 2
	bear.TurnedFaceUpThisTurn = true
	if walker.ActivatedThisTurn != 2 || !bear.TurnedFaceUpThisTurn {
		t.Fatalf("precondition: the facts did not stick")
	}
	cases := []struct {
		spec string
		key  string
		want bool
	}{
		{"Planeswalker.activated", "myWalker", true},
		{"Creature.activated", "myBear", false},           // the bear activated nothing
		{"Planeswalker.!activated", "myWalker", false},    // the negation inverts
		{"Creature.turnedFaceUpThisTurn", "myBear", true}, // the bear was turned up
		{"Creature.!turnedFaceUpThisTurn", "myBear", false},
		{"Planeswalker.turnedFaceUpThisTurn", "myWalker", false}, // the planeswalker was not
	}
	for _, c := range cases {
		if got := MatchesSpec(g, c.spec, id[c.key], 0); got != c.want {
			t.Errorf("%s: %v, want %v", c.spec, got, c.want)
		}
	}

	// Draining the facts flips the reads back (the same fields TurnChange
	// and the zone clear reset; engine/events pins those folds).
	walker.ActivatedThisTurn = 0
	bear.TurnedFaceUpThisTurn = false
	if MatchesSpec(g, "Planeswalker.activated", walker.ID, 0) {
		t.Errorf("activated kept matching after the tally drained")
	}
	if MatchesSpec(g, "Creature.turnedFaceUpThisTurn", bear.ID, 0) {
		t.Errorf("turnedFaceUpThisTurn kept matching after the mark cleared")
	}
}
