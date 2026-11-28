package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// drawn_owner_scope_referent_test.go pins the bare-base player control
// referent `Opponent.NonActive` (Forge writes `Opponent.!Active`, which
// engine/cards/parse.go's canonicalNonActive folds): Forge's CardProperty
// `OwnedBy` applied to the shared PlayerProperty grammar without the Player.
// prefix, the Opponent base beside the NonActive qualifier that
// player_filter.go already reads. The corpus carrier is Tataru Taru's Scions'
// Secretary (.cards/cardsfolder/t/tataru_taru.txt, `T:Mode$ Drawn | ValidCard$
// Card.OwnedBy Opponent.!Active | TriggerZones$ Battlefield | Execute$
// TrigToken ... create a tapped Treasure token`): Forge matches a Drawn
// trigger's ValidCard$ against the drawn card and CardProperty `OwnedBy`
// against that card's OWNER, who is the drawing player (CR 108.4a), so the
// scope reads "an opponent of the controller who is not the active player".
// Until the referent tables knew the two-word spelling the ValidCard$ matched
// nothing and the ability never triggered.

func TestControlReferentOpponentNonActiveMatchesOffTurnOpponents(t *testing.T) {
	g, _ := board(t)
	sc := SpecContext{You: 0}

	// PRECONDITION: the rule reads the OWNER, so owner and controller differ
	// on every object below and an OwnedBy match cannot be a ControlledBy
	// match in disguise.
	opponentOwned := &state.Object{Owner: 1, Controller: 0}
	controllerOwned := &state.Object{Owner: 0, Controller: 1}
	if opponentOwned.Owner == opponentOwned.Controller ||
		opponentOwned.Owner == controllerOwned.Owner {
		t.Fatal("precondition: compared owner/control values must differ")
	}

	// PRECONDITION: the shared player grammar the referent delegates to
	// really separates the seats by whose turn it is, so a match below is
	// not the referent matching on an empty qualifier.
	g.Active = 0
	if !MatchesPlayerSpecCtx(g, "Opponent.NonActive", 1, 0, PlayerSpecCtx{}) {
		t.Fatal("precondition: seat 1 off turn is not the non-active opponent of seat 0")
	}
	if MatchesPlayerSpecCtx(g, "Opponent.NonActive", 0, 0, PlayerSpecCtx{}) {
		t.Fatal("precondition: seat 0 must not be its own opponent")
	}

	for _, spec := range []string{
		"Card.OwnedBy Opponent.NonActive",
		"Card.!OwnedBy Opponent.NonActive",
	} {
		if unknown := UnknownPredicates(spec); len(unknown) != 0 {
			t.Fatalf("recognized referent %s reported unknown: %v", spec, unknown)
		}
	}

	// The controller's own turn: seat 1's draw is the off-turn one Tataru
	// triggers on.
	if !MatchesObjectCtx(g, "Card.OwnedBy Opponent.NonActive", opponentOwned, sc) {
		t.Fatal("a card owned by an opponent off turn did not match")
	}
	if MatchesObjectCtx(g, "Card.OwnedBy Opponent.NonActive", controllerOwned, sc) {
		t.Fatal("the controller's own card matched the non-active opponent referent")
	}
	if !MatchesObjectCtx(g, "Card.!OwnedBy Opponent.NonActive", controllerOwned, sc) {
		t.Fatal("negated ownership did not match a bound different owner")
	}
	if MatchesObjectCtx(g, "Card.ControlledBy Opponent.NonActive", opponentOwned, sc) {
		t.Fatal("OwnedBy referent incorrectly compared the candidate controller")
	}

	// Tataru's own gate: while seat 1's turn runs, that opponent IS the
	// active player, so the same card no longer matches.
	g.Active = 1
	if MatchesObjectCtx(g, "Card.OwnedBy Opponent.NonActive", opponentOwned, sc) {
		t.Fatal("the active player's own draw matched the off-turn referent")
	}
	g.Active = 0

	// The referent's own binding is the same set, and unbound -- never a
	// guess -- when no seat qualifies.
	players, ok := controlReferentPlayers(g, sc, "OwnedBy", "Opponent.NonActive")
	if !ok || len(players) != 1 || players[0] != 1 {
		t.Fatalf("controlReferentPlayers(OwnedBy, Opponent.NonActive) = %v, %v; want [1], true", players, ok)
	}
	g.Active = 1
	if bound, ok := controlReferentPlayers(g, sc, "OwnedBy", "Opponent.NonActive"); ok {
		t.Fatalf("with the only opponent active the referent bound %v; want unbound", bound)
	}
}
