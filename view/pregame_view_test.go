// This file is package view_test (same reason as describe_replay_test.go:
// the pregame projection is a rules-and-view contract, and rules cannot be
// imported from an internal view test file without an import cycle). It
// pins the rv2a pregame fixes: during the London mulligan round the
// projected active seat is the CR 103.1 toss winner -- never the seat-0
// zero value -- and the toss transcript renders through the seat's display
// name like every other seat-naming line.
package view_test

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// mountainDecks builds n identical 40-Mountain constructed decks, the same
// fixture shape rules/starting_player_test.go uses -- no corpus needed.
func mountainDecks(t *testing.T, n int) [][]*cards.Card {
	t.Helper()
	src := "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	out := make([][]*cards.Card, n)
	for i := range out {
		deck := make([]*cards.Card, 40)
		for j := range deck {
			deck[j] = c
		}
		out[i] = deck
	}
	return out
}

// tossedEngine builds a two-seat London-mulligan game whose toss (measured
// by the returned engine's own pre-deal Note) went to wantToss. Seat
// display names come from playerNames so the describe assertions can tell
// the display name from the deck identity the chain carries.
func tossedEngine(t *testing.T, seed uint64, mulligans int, playerNames []string) *rules.Engine {
	t.Helper()
	e := rules.New(rules.Config{Seed: seed, Names: []string{"a", "b"},
		Decks: mountainDecks(t, 2), Mulligans: mulligans, PlayerNames: playerNames})
	for {
		won := false
		for _, ev := range e.L.Events {
			if ev.Kind == events.Note && ev.Text == "b won the toss" {
				if ev.Player == 1 {
					won = true
				}
				break
			}
		}
		if won {
			return e
		}
		seed++
		e = rules.New(rules.Config{Seed: seed, Names: []string{"a", "b"},
			Decks: mountainDecks(t, 2), Mulligans: mulligans, PlayerNames: playerNames})
	}
}

// TestPregameViewProjectsTheTossWinnerAsActive is the rv2a pregame gate:
// during the mulligan round the projected active seat is the toss winner
// for a seed where seat 1 wins -- the pre-toss engine's view reported the
// seat-0 zero value here, and the board marked the wrong player active
// while the toss Note named another.
func TestPregameViewProjectsTheTossWinnerAsActive(t *testing.T) {
	e := tossedEngine(t, 1, 1, []string{"Alice", "Bob"})
	if e.G.Turn != 0 {
		t.Fatalf("fixture precondition: turn=%d, want pregame", e.G.Turn)
	}
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KMulligan {
		t.Fatalf("pending = %+v, want the mulligan round's first ask", d)
	}
	for _, viewer := range []state.PlayerID{0, 1} {
		v := view.Project(e.G, e, viewer, d)
		if v.Active != 1 {
			t.Fatalf("viewer %d: pregame view active = %d, want the toss winner (seat 1)", viewer, v.Active)
		}
		if v.Turn != 0 {
			t.Fatalf("viewer %d: pregame view turn = %d, want 0", viewer, v.Turn)
		}
	}
}

// TestPregameViewReportsNoActiveSeatWhenNoTurnBegan: a game the opening
// deal ended never began a turn, so its view has no active seat -- not the
// seat-0 zero value.
// TestLiveViewPreservesPriority guards the normal turn projection against
// pregame handling: only Active is special during a mulligan round. A live
// seat's priority is always the event-folded g.Priority, including seat 1.
func TestLiveViewPreservesPriority(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	g.Turn, g.Active, g.Priority = 1, 1, 1
	v := view.Project(g, nil, 0, nil)
	if v.Priority != 1 {
		t.Fatalf("live view priority = %d, want 1", v.Priority)
	}
}

func TestPregameViewReportsNoActiveSeatWhenNoTurnBegan(t *testing.T) {
	decks := mountainDecks(t, 2)
	decks[1] = decks[1][:3]
	e := rules.New(rules.Config{Seed: 1, Names: []string{"a", "b"}, Decks: decks})
	if !e.G.Over || e.G.Turn != 0 {
		t.Fatalf("fixture precondition: over=%v turn=%d, want a terminal genesis", e.G.Over, e.G.Turn)
	}
	v := view.Project(e.G, e, 0, nil)
	if v.Active != view.NoSeat {
		t.Fatalf("terminal genesis view active = %d, want no active seat (%d)", v.Active, view.NoSeat)
	}
}

// TestDescribeRendersTheTossNoteThroughThePlayerName: the toss Note's chain
// text carries the deck identity (F3 keeps display names out of the chain),
// but the transcript line reads it through player() -- the same label the
// player box shows -- instead of echoing the raw chain text. Before this
// the transcript read "a won the toss" beside "Alice is asked".
func TestDescribeRendersTheTossNoteThroughThePlayerName(t *testing.T) {
	e := tossedEngine(t, 1, 1, []string{"Alice", "Bob"})
	var note events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Text == "b won the toss" {
			note = ev
			break
		}
	}
	if note.Text != "b won the toss" {
		t.Fatalf("fixture precondition: chain text %q", note.Text)
	}
	// The line names the seat the way the player box does -- the seat's own
	// display name -- even though the chain text carries the deck identity.
	if got, want := view.Describe(e.G, note), "Bob won the toss"; got != want {
		t.Fatalf("describe toss note = %q, want %q", got, want)
	}
}

// TestDescribeLeavesANonGenesisTossPhraseAlone proves the renderer recognizes
// the genesis event's structural position as well as its text. A later card
// effect is allowed to use the exact same words and seat-bound deck identity;
// it must remain verbatim rather than being rewritten through PlayerName.
func TestDescribeLeavesANonGenesisTossPhraseAlone(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	g.Players[0].PlayerName = "Alice"
	ev := events.Event{Seq: 9, Kind: events.Note, Player: 0, Text: "a won the toss"}
	if got, want := view.Describe(g, ev), ev.Text; got != want {
		t.Fatalf("non-genesis toss phrase = %q, want %q", got, want)
	}
}
