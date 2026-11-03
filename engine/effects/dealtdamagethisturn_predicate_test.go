package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDealtDamageThisTurnPredicate is the effects leaf for Forge's
// dealtDamageThisTurn -- the SOURCE side of combat damage, distinct from
// wasDealtDamageThisTurn (the recipient side). It reads
// state.Object.DealtDamageThisTurnHere, the flag events.Apply's
// DamageProvenance fold sets. Corpus carriers: Avenging Arrow,
// Executioner's Swing, Restore the Peace, Red Guardian, Super Soldier and
// Treacherous Greed's sacrifice cost. Asserting the precondition (the two
// candidate objects actually differ in the record) keeps the test from
// passing vacuously if the predicate matched nothing.
func TestDealtDamageThisTurnPredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	dealt := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	dealt.DealtDamageThisTurnHere = true
	silent := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	// A creature that dealt damage, left and came back keeps the turn-wide
	// ledger, but it is a new object that has dealt none (CR 400.7).
	silent.DamageDealtThisTurn = append(silent.DamageDealtThisTurn, state.DamageDealtRecord{Amount: 2})

	if !MatchesObjectCtx(g, "Creature.dealtDamageThisTurn", dealt, SpecContext{You: 0}) {
		t.Error("Creature.dealtDamageThisTurn must match a creature that dealt damage this turn")
	}
	if MatchesObjectCtx(g, "Creature.dealtDamageThisTurn", silent, SpecContext{You: 0}) {
		t.Error("Creature.dealtDamageThisTurn must not match a creature that dealt no damage this turn")
	}
	// The census and the matcher share the one recogniser: an unknown-word
	// regression here would silently make every carrier's filter inert.
	if got := UnknownPredicates("Creature.dealtDamageThisTurn"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Creature.dealtDamageThisTurn) = %v, want empty", got)
	}
	// The recipient-side sibling stays a separate predicate (a creature that
	// was dealt damage is not thereby one that dealt damage).
	if MatchesObjectCtx(g, "Creature.wasDealtDamageThisTurn", silent, SpecContext{You: 0}) {
		t.Error("dealtDamageThisTurn must not widen wasDealtDamageThisTurn")
	}
}
