package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// TestDiscardRevealYouChooseAsksTheCasterNotTheTarget pins the chooser/target
// split that is the whole point of the fix: for a Mode$ RevealYouChoose
// discard, the CASTER (seat 0, Ctx.Controller) makes the decision — never the
// player whose card is being discarded — and the answer is honoured: the
// chosen card leaves the hand, not hand[0]. The CHOSEN option is index 1
// (bird), deliberately not hand[0] (frog), to prove the choice is real.
//
// This test lives in its own file because it is the one that needs the
// continuation's answer field (Ctx.Discard, the shape the engine's "discard"
// resume arm fills) to simulate a re-entry. Against the base Ctx — which has
// no way to carry "which object" — it cannot even compile, which is itself
// conclusive that the base code cannot express the answer.
func TestDiscardRevealYouChooseAsksTheCasterNotTheTarget(t *testing.T) {
	ah, ctx, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"))
	s := sa(t, "SP$ Discard | ValidTgts$ Player | Mode$ RevealYouChoose | DiscardValid$ Card | NumCards$ 1")

	effDiscard(ah, ctx, s)

	if ah.asked == nil {
		t.Fatal("RevealYouChoose posed no decision")
	}
	if ah.asked.Player != 0 {
		t.Fatalf("chooser = seat %d, want the caster seat 0", ah.asked.Player)
	}
	if ah.asked.Kind != decision.KModes {
		t.Fatalf("decision kind = %s, want KModes", ah.asked.Kind)
	}
	if ah.asked.Min != 1 || ah.asked.Max != 1 {
		t.Fatalf("Min/Max = %d/%d, want 1/1", ah.asked.Min, ah.asked.Max)
	}
	if len(ah.asked.Options) != 2 {
		t.Fatalf("options = %d, want the 2 hand cards", len(ah.asked.Options))
	}
	chosen := ah.asked.Options[1]
	if chosen.Obj != ids[1] {
		t.Fatalf("option 1 obj = %d, want bird %d", chosen.Obj, ids[1])
	}

	// Simulate the engine's resume: the continuation set Ctx.Discard to the
	// chosen object, then re-runs the effect.
	ctx.Discard = []state.ObjID{chosen.Obj}
	effDiscard(ah, ctx, s)

	if !inZone(ah.g, state.ZGraveyard, 1, ids[1]) {
		t.Fatal("the chosen card (bird) was not moved to the graveyard")
	}
	if inZone(ah.g, state.ZHand, 1, ids[1]) {
		t.Fatal("the chosen card is still in hand")
	}
	if !inZone(ah.g, state.ZHand, 1, ids[0]) {
		t.Fatal("the un-chosen card (frog) left the hand — the choice was ignored")
	}
}
