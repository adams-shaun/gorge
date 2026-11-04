package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestYouCastThisGameGatesAlternativeCost pins the second carrier of
// Count$YouCastThisGame: Once Upon a Time's free-cast alternative cost is
// gated by an SVar compared to zero, which the count now evaluates for real.
// With nothing in the pool the card is castable only while the controller has
// cast no spell this game; after a first spell it is not offered.
func TestYouCastThisGameGatesAlternativeCost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ouat := lookup(t, reg, "Once Upon a Time")
	filler := card(t, firstContactFillerSrc)
	setup := func(t *testing.T) *Engine {
		e, _ := corpusEngineCfg(t, reg, []*cards.Card{ouat, filler, filler}, nil)
		moveByName(t, e, 0, "Once Upon a Time", state.ZHand)
		moveByName(t, e, 0, "Test Filler Instant", state.ZHand)
		return e
	}
	t.Run("free cast is offered before any spell is cast", func(t *testing.T) {
		e := setup(t)
		if got := e.SpellsCastThisGame(0); got != 0 {
			t.Fatalf("precondition: SpellsCastThisGame = %d, want 0", got)
		}
		if got := castOptionsFor(e, findInHand(e, 0, "Once Upon a Time")); len(got) == 0 {
			t.Fatal("Once Upon a Time with an empty pool is not offered as the first spell")
		}
	})
	t.Run("free cast is gone after a spell was cast", func(t *testing.T) {
		e := setup(t)
		addMana(t, e, 0, "R")
		castNamed(t, e, "Test Filler Instant")
		drainFirstContact(t, e)
		if got := e.SpellsCastThisGame(0); got != 1 {
			t.Fatalf("precondition: SpellsCastThisGame = %d, want 1", got)
		}
		if got := castOptionsFor(e, findInHand(e, 0, "Once Upon a Time")); len(got) != 0 {
			t.Fatalf("Once Upon a Time is offered free as the second spell: %+v", got)
		}
	})
}
