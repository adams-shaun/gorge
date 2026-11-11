package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestWindfallDrawsTheGreatestDiscard pins the DRAW half of Windfall end to
// end: each player discards their hand (8 for seat 0, 7 for seat 1), then both
// draw the greatest number discarded this way (8), through the real
// PlayerCountPlayers$HighestValidGraveyard,Library,Exile Card.IsRemembered+
// YouOwn SVar. Before the multi-zone count was read, X degraded to 0 and each
// player drew nothing.
func TestWindfallDrawsTheGreatestDiscard(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := wheelEngine(t, reg, "Windfall")

	// Give seat 0 one extra card (a logged library->hand move, the same
	// primitive searchMoveByName uses) so the greatest discarded count (8)
	// differs from seat 1's discard (7): the draw count has to be the
	// greatest, not each player's own.
	extra := searchMoveByName(t, e, "Forest", state.ZHand)
	if extra == 0 {
		t.Fatal("no Forest to move into seat 0's hand")
	}
	before0 := len(e.G.Zone(state.ZHand, 0))
	before1 := len(e.G.Zone(state.ZHand, 1))
	if before0 != 8 || before1 != 7 {
		t.Fatalf("pre-cast hands: seat0 %d, seat1 %d -- fixture expects 8/7", before0, before1)
	}
	// Snapshot the log's draws before the cast, so the opening hand (also
	// recorded as Draw events) does not inflate the wheel's own draw count.
	drawsBefore0, drawsBefore1 := countDraws(e, 0), countDraws(e, 1)

	castWheel(t, e, "Windfall", "UUU")

	// Both players discarded their whole hand (15 cards) ...
	if got := len(wheelDiscards(e)); got != before0+before1 {
		t.Fatalf("discard events = %d, want %d", got, before0+before1)
	}
	// ... and each drew the GREATEST discarded count (8), not their own.
	if got := len(e.G.Zone(state.ZHand, 0)); got != before0 {
		t.Fatalf("seat 0 hand after Windfall = %d, want %d (the greatest discard)", got, before0)
	}
	if got := len(e.G.Zone(state.ZHand, 1)); got != before0 {
		t.Fatalf("seat 1 hand after Windfall = %d, want %d (the greatest discard, not its own 7)", got, before0)
	}

	// The draws are real card draws: one events.Draw per card drawn, never a
	// zero-amount draw no-op.
	if got := countDraws(e, 0) - drawsBefore0; got != before0 {
		t.Fatalf("seat 0 draw events = %d, want %d (the greatest discard)", got, before0)
	}
	if got := countDraws(e, 1) - drawsBefore1; got != before0 {
		t.Fatalf("seat 1 draw events = %d, want %d (the greatest discard, not its own 7)", got, before0)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Draw && ev.Obj == 0 {
			t.Fatal("a draw event carried no card; the count SVar must resolve to the greatest discarded")
		}
	}
}
