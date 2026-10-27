package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The `Card.OwnedBy|ControlledBy Player.<qualifier>` control/ownership
// referent is Forge's player-property grammar applied to the candidate's
// owner/controller (CardProperty.OwnedBy/ControlledBy tests
// player.isValid first). Before cr-g242 only a hand-listed few Player.*
// spellings were admitted, so Wrexial's `Instant.OwnedBy
// Player.TriggeredTarget` offered no target and Ashiok's, Marisi's and
// Glen Elendra's player-scoped filters matched nothing. These tests pin the
// shared player-spec delegation and its fail-closed unknown boundary.

// TestControlReferentPlayerOpponentMatchesOpponentsCards: a
// `Player.Opponent` owner is an opponent of the evaluating controller, and
// not the controller itself. The controller is flipped from the owner so a
// match cannot come from confusing OwnedBy with ControlledBy.
func TestControlReferentPlayerOpponentMatchesOpponentsCards(t *testing.T) {
	g, _ := board(t)
	sc := SpecContext{You: 0}

	// PRECONDITION: the shared player grammar really reads seat 1 as the
	// opponent of seat 0, so the referent below is not matching on an empty
	// qualifier.
	if !MatchesPlayerSpecCtx(g, "Player.Opponent", 1, 0, PlayerSpecCtx{}) {
		t.Fatal("precondition: seat 1 is not the opponent of seat 0")
	}
	if MatchesPlayerSpecCtx(g, "Player.Opponent", 0, 0, PlayerSpecCtx{}) {
		t.Fatal("precondition: seat 0 must not be its own opponent")
	}

	opponentOwned := &state.Object{Owner: 1, Controller: 0}
	controllerOwned := &state.Object{Owner: 0, Controller: 1}
	// PRECONDITION: owner and controller differ, so an OwnedBy match cannot be
	// a ControlledBy match in disguise.
	if opponentOwned.Owner == opponentOwned.Controller ||
		opponentOwned.Owner == controllerOwned.Owner {
		t.Fatal("precondition: compared owner/control values must differ")
	}

	for _, spec := range []string{
		"Card.OwnedBy Player.Opponent",
		"Card.!OwnedBy Player.Opponent",
	} {
		if unknown := UnknownPredicates(spec); len(unknown) != 0 {
			t.Fatalf("recognized referent %s reported unknown: %v", spec, unknown)
		}
	}
	if !MatchesObjectCtx(g, "Card.OwnedBy Player.Opponent", opponentOwned, sc) {
		t.Fatal("an opponent's owned card did not match despite a different controller")
	}
	if MatchesObjectCtx(g, "Card.OwnedBy Player.Opponent", controllerOwned, sc) {
		t.Fatal("the controller's own card matched the opponent referent")
	}
	if !MatchesObjectCtx(g, "Card.!OwnedBy Player.Opponent", controllerOwned, sc) {
		t.Fatal("negated ownership did not match a bound different owner")
	}
	if MatchesObjectCtx(g, "Card.ControlledBy Player.Opponent", opponentOwned, sc) {
		t.Fatal("OwnedBy referent incorrectly compared the candidate controller")
	}
}

// TestControlReferentUnknownPlayerQualifierFailsClosed: a qualifier the shared
// player grammar does not read is still unknown to UnknownPredicates and
// matches nobody, including beneath negation.
func TestControlReferentUnknownPlayerQualifierFailsClosed(t *testing.T) {
	g, _ := board(t)
	sc := SpecContext{You: 0}
	owner := &state.Object{Owner: 1, Controller: 0}

	for _, spec := range []string{
		"Card.OwnedBy Player.NoSuchThing",
		"Card.!OwnedBy Player.NoSuchThing",
	} {
		if unknown := UnknownPredicates(spec); len(unknown) == 0 {
			t.Fatalf("%s should stay unknown, got no unknown report", spec)
		}
		if MatchesObjectCtx(g, spec, owner, sc) {
			t.Fatalf("%s matched despite an unknown qualifier", spec)
		}
	}
}
