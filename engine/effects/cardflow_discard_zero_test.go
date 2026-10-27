package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// corpusDiscardSA loads a REAL compiled corpus card and returns the first
// Discard SA on it whose Mode$ is mode, together with the face whose SVars
// the resolution reads.
func corpusDiscardSA(t *testing.T, name, mode string) (*cards.SA, *cards.Face) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus has no card %q", name)
	}
	if len(card.Faces) == 0 {
		t.Fatalf("card %q has no faces", name)
	}
	f := card.Faces[0]
	s := findDiscardMode(f, mode)
	if s == nil {
		t.Fatalf("card %q has no %s Discard ability", name, mode)
	}
	return s, f
}

// discardMovesFor returns the hand-to-graveyard MoveZone events (the shape
// events.Discard emits) for player p.
func discardMovesFor(log []events.Event, p state.PlayerID) []events.Event {
	var out []events.Event
	for _, e := range log {
		if e.Kind == events.MoveZone && e.From == state.ZHand && e.To == state.ZGraveyard && e.Player == p {
			out = append(out, e)
		}
	}
	return out
}

// assertNoDiscardZero pins the "resolved zero discards nothing" outcome: no
// posed decision, no fallback Note, no discard move, and every hand card
// still in hand. It also checks the precondition that the hand is non-empty
// and eligible, so the zero cannot be the earlier len(eligible) == 0 skip.
func assertNoDiscardZero(t *testing.T, ah *askHost, s *cards.SA, c *Ctx, ids []state.ObjID) {
	t.Helper()
	if got := Num(ah, c, s, "NumCards", 1); got != 0 {
		t.Fatalf("precondition: NumCards resolved to %d, want 0", got)
	}
	valid := s.ParamStr(cards.PKDiscardValid)
	if valid == "" {
		valid = "Card"
	}
	if got := len(discardEligible(ah.g, c, ids, valid)); got != len(ids) {
		t.Fatalf("precondition: %d of %d hand cards eligible, want all", got, len(ids))
	}

	effDiscard(ah, c, s)

	if ah.asked != nil {
		t.Fatalf("zero discard posed a decision: kind=%v min=%d max=%d prompt=%q",
			ah.asked.Kind, ah.asked.Min, ah.asked.Max, ah.asked.Prompt)
	}
	if moves := discardMovesFor(ah.log, 1); len(moves) != 0 {
		t.Fatalf("zero discard moved %d card(s): %+v", len(moves), moves)
	}
	for _, id := range ids {
		if !inZone(ah.g, state.ZHand, 1, id) {
			t.Fatalf("card %d left hand on a zero discard", id)
		}
	}
	for _, e := range ah.log {
		if e.Kind == events.Note && strings.Contains(e.Text, "no engine host to ask") {
			t.Fatalf("zero discard emitted the no-host fallback Note %q", e.Text)
		}
	}
}

// TestDiscardComputedZeroTgtChooseDiscardsNothing pins Sink into Takenuma's
// TgtChoose SA with an empty remembered set: NumCards$ Y resolves to zero, so
// Forge's DiscardEffect takes max == 0 (CardCollection.EMPTY) and poses no
// chooseCardsToDiscardFrom ask.
func TestDiscardComputedZeroTgtChooseDiscardsNothing(t *testing.T) {
	s, f := corpusDiscardSA(t, "Sink into Takenuma", "TgtChoose")
	ah, c, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"), creature(t, "Cat"))
	c.SVars = f.SVars
	// Remembered is empty: no Swamps were swept, so Y = Remembered$Amount = 0.

	assertNoDiscardZero(t, ah, s, c, ids)
}

// TestDiscardComputedZeroChooseModeDiscardsNothing is the same zero rule on
// the choose-mode call site: Last Rites' Mode$ RevealYouChoose SA with an
// empty remembered set (the first half discarded zero cards).
func TestDiscardComputedZeroChooseModeDiscardsNothing(t *testing.T) {
	s, f := corpusDiscardSA(t, "Last Rites", "RevealYouChoose")
	ah, c, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"), creature(t, "Cat"))
	c.SVars = f.SVars
	c.Remembered = nil

	assertNoDiscardZero(t, ah, s, c, ids)
}

// TestDiscardComputedNonzeroDiscardsExactlyThatMany is the non-zero control:
// the same Sink into Takenuma SA with two remembered cards must still ask
// 2..2 and discard exactly two, so the zero fix does not weaken the ordinary
// computed count.
func TestDiscardComputedNonzeroDiscardsExactlyThatMany(t *testing.T) {
	s, f := corpusDiscardSA(t, "Sink into Takenuma", "TgtChoose")
	ah, c, ids := discardBoard(t, creature(t, "Frog"), creature(t, "Bird"), creature(t, "Cat"))
	c.SVars = f.SVars
	c.Remembered = []state.Target{{Obj: ids[0]}, {Obj: ids[1]}}

	if got := Num(ah, c, s, "NumCards", 1); got != 2 {
		t.Fatalf("precondition: NumCards resolved to %d, want 2", got)
	}

	effDiscard(ah, c, s)

	if ah.asked == nil {
		t.Fatal("non-zero discard posed no decision")
	}
	if ah.asked.Min != 2 || ah.asked.Max != 2 {
		t.Fatalf("asked %d..%d, want 2..2 (%q)", ah.asked.Min, ah.asked.Max, ah.asked.Prompt)
	}
	moves := discardMovesFor(ah.log, 1)
	if len(moves) != 2 {
		t.Fatalf("%d discard move(s), want exactly 2: %+v", len(moves), moves)
	}
	gone := 0
	for _, id := range ids {
		if inZone(ah.g, state.ZGraveyard, 1, id) {
			gone++
		}
	}
	if gone != 2 {
		t.Fatalf("%d card(s) in the graveyard, want exactly 2", gone)
	}
	if !inZone(ah.g, state.ZHand, 1, ids[2]) {
		t.Fatal("the third hand card was discarded; the count was not respected")
	}
}
