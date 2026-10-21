package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAkimFirstTokenCreationOnly pins Akim, the Soaring Wind's OnlyFirst$ You
// rider to Forge's TriggerTokenCreatedOnce.performTest: the line fires only
// when the token's creator is the player named by OnlyFirst$ AND that creation
// is that player's first token creation of the turn. A per-turn trigger cap
// cannot express either half: it fires on an opponent's first creation, and it
// still fires on the controller's later creation after tokens were made before
// Akim entered.
func TestAkimFirstTokenCreationOnly(t *testing.T) {
	t.Parallel()

	// (a) An opponent's first creation does not fire Akim; the controller's
	// own first creation later in the same turn does.
	t.Run("opponent_first_then_controller_first", func(t *testing.T) {
		akim := tokenReplCorpusCard(t, "Akim, the Soaring Wind")
		maker0 := cardByName(t, tokenForgeSrc("c_a_food_sac"))
		maker1 := cardByName(t, tokenForgeSrc("c_a_food_sac"))
		e, cfg := tokenReplGameSeats(t, 41, []*cards.Card{akim, maker0}, []*cards.Card{maker1})
		akimID := moveSeededCard(t, e, 0, akim, state.ZBattlefield)
		maker0ID := moveSeededCard(t, e, 0, maker0, state.ZBattlefield)
		maker1ID := moveSeededCard(t, e, 1, maker1, state.ZBattlefield)
		if o := e.G.Obj(akimID); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Akim is not on the battlefield: %+v", o)
		}
		addMana(t, e, 0, "") // re-ask priority after the seeded moves

		// Seat 1's turn: the opponent creates the turn's first token.
		driveToStep(t, e, 2, 1, state.StepMain1)
		addMana(t, e, 1, "")
		submitChoices(t, e, abilityOption(t, e, maker1ID, 0).Index)
		passUntilStackEmpty(t, e, 20)
		if got := countTokensNamedOnSeat(t, e, 1, "Food Token"); got != 1 {
			t.Fatalf("precondition: opponent Food Tokens = %d, want 1", got)
		}
		if got := countTokensNamedOnSeat(t, e, 0, "Bird Token"); got != 0 {
			t.Fatalf("Akim fired on the opponent's token creation: Bird Tokens = %d, want 0", got)
		}

		// Seat 0's turn: its own first creation still fires Akim.
		driveToStep(t, e, 3, 0, state.StepMain1)
		addMana(t, e, 0, "")
		submitChoices(t, e, abilityOption(t, e, maker0ID, 0).Index)
		passUntilStackEmpty(t, e, 20)
		if got := countTokensNamedOnSeat(t, e, 0, "Bird Token"); got != 1 {
			t.Fatalf("Akim did not fire on the controller's first creation: Bird Tokens = %d, want 1", got)
		}
		replayCheck(t, e, cfg)
	})

	// (b) Tokens created earlier in the turn, before Akim entered, make the
	// controller's later creation not the first, so Akim does not fire.
	t.Run("controller_created_before_akim_entered", func(t *testing.T) {
		akim := tokenReplCorpusCard(t, "Akim, the Soaring Wind")
		maker := cardByName(t, tokenForgeSrc("c_a_food_sac"))
		e, cfg := tokenReplGame(t, 41, akim, maker)
		makerID := moveSeededCard(t, e, 0, maker, state.ZBattlefield)

		activateTokenForge(t, e, makerID) // first creation, with Akim not yet on the battlefield
		if got := countTokensNamedOnSeat(t, e, 0, "Food Token"); got != 1 {
			t.Fatalf("precondition: Food Tokens = %d, want 1", got)
		}
		if got := countTokensNamedOnSeat(t, e, 0, "Bird Token"); got != 0 {
			t.Fatalf("precondition: Bird Tokens = %d, want 0 before Akim enters", got)
		}

		moveSeededCard(t, e, 0, akim, state.ZBattlefield)
		untapUntaps(t, e, makerID)
		activateTokenForge(t, e, makerID) // not the controller's first creation this turn
		if got := countTokensNamedOnSeat(t, e, 0, "Food Token"); got != 2 {
			t.Fatalf("precondition: Food Tokens = %d, want 2", got)
		}
		if got := countTokensNamedOnSeat(t, e, 0, "Bird Token"); got != 0 {
			t.Fatalf("Akim fired on a later creation: Bird Tokens = %d, want 0", got)
		}
		replayCheck(t, e, cfg)
	})
}
