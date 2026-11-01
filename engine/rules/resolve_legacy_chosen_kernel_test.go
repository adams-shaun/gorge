package rules

// Three cardfuzz shapes: the chosen-card binding, the Choice channel and the
// unless payer cursor. Each runs on the kernel and pins the game outcome.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Whiskervale Forerunner's shape: the chosen card fetched to the battlefield
// past an Optional$ confirmation keeps its binding, so only the EQ1 leg
// gains (1).
func TestKernelChosenBindingAcrossAsk(t *testing.T) {
	var life int32
	e, _ := runScenarioKernel(t, 2, 33417, func(t *testing.T, e *Engine) {
		life = e.G.Players[0].Life
		legacyChosenScenario(t, e)
	}, legacyChosenAcrossAskSrc)
	if got := e.G.Players[0].Life - life; got != 1 {
		t.Fatalf("the chosen card went to the battlefield, so only the EQ1 leg gains (1); gained %d", got)
	}
}

// Shrouded Lore's shape: a second ChooseCard replaces the first's choice, so
// Defined$ ChosenCard fetches exactly the last choice's one card.
func TestKernelChooseCardRecordLeavesNoStaleAnswer(t *testing.T) {
	var hand int
	e, _ := runScenarioKernel(t, 2, 33418, func(t *testing.T, e *Engine) {
		hand = len(e.G.Zone(state.ZHand, 0))
		tapeCastAndResolve(t, e, "Tape Twice Chosen", "B")
	}, legacyStaleChoiceSrc)
	if got := len(e.G.Zone(state.ZHand, 0)) - (hand - 1); got != 1 {
		t.Fatalf("Defined$ ChosenCard fetched %d cards, want only the last choice's 1", got)
	}
}

// Rhystic Circle's shape: every payer declines the unless cost, so the body
// chooses a land (its own chooser walk starting at zero) and fetches it.
func TestKernelUnlessElectionCursorDoesNotReachTheBody(t *testing.T) {
	var hand int
	e, _ := runScenarioKernel(t, 2, 33419, func(t *testing.T, e *Engine) {
		hand = len(e.G.Zone(state.ZHand, 0))
		tapeUnlessScenario("Tape Rhystic Pick", "B", 0, tapeUnlessPick(false))(t, e)
	}, legacyUnlessCursorSrc)
	if got := len(e.G.Zone(state.ZHand, 0)) - (hand - 1); got != 1 {
		t.Fatalf("every payer declined, so the body chooses a land and fetches it; hand gained %d", got)
	}
}
