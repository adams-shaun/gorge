package host

// CR 723.4 host wiring test: projectNext must widen the projected View the
// controller receives to include the hidden information of the seats they
// control. rules' controlPlayerRedirect rewrites a controlled seat's decision
// to the controller, so by the time the host projects, d.Player IS the
// controller; projectNext reverses state.Game.ControlledBy to find the
// controlled seats and passes them as the view's alsoVisible set. Without
// that, the controller answers the controlled seat's decision blind (the
// controlled hand is a count) -- the CR 723.4 gap. A third seat's hand must
// stay hidden.

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestProjectNextRevealsControlledHandToController drives a live 3-seat table
// whose seat 0 is human, parks on seat 0's first decision, folds a CR 723
// control grant (seat 1 controlled by seat 0) into the engine, and projects
// the next decision through the real projectNext. The controller reads seat
// 1's hand; seat 2's stays hidden.
func TestProjectNextRevealsControlledHandToController(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.Seats = func(names []string, seed uint64) []seat.Seat {
		out := make([]seat.Seat, len(names))
		out[0] = NewHumanSeat()
		for i := 1; i < len(out); i++ {
			out[i] = heuristic.NewBot(seed ^ uint64(i))
		}
		return out
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := TableConfig{ID: "t1", Name: "human", Seats: 3, Decks: []string{"a", "b", "c"},
		Seed: 42, Pace: 0, Spectator: view.Omniscient, Perpetual: false}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	// Park on seat 0's first decision so the match goroutine is not driving
	// the engine while we project (it is blocked in the human seat's park).
	waitPending(t, r, "t1", 1, 0)

	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	if tb.cur == nil {
		t.Fatal("no live match")
	}
	m := tb.cur
	if m.e == nil || m.e.Pending() == nil {
		t.Fatal("precondition: live match with a pending decision")
	}
	d := m.e.Pending()
	if d.Player != 0 {
		t.Fatalf("precondition: seat 0's decision pending, got player %d", d.Player)
	}
	// Precondition: seat 1 really holds cards, or the positive assertion below
	// would pass on an empty hand of the wrong shape.
	if len(m.e.G.Zone(state.ZHand, 1)) == 0 {
		t.Fatal("precondition: seat 1 has no hand to reveal")
	}
	if len(m.e.G.Zone(state.ZHand, 2)) == 0 {
		t.Fatal("precondition: seat 2 has no hand to keep hidden")
	}
	// Fold a CR 723 grant: seat 0 controls seat 1. Written directly into the
	// engine's fold maps (this is the state the ControlPlayerChange fold would
	// have produced) rather than driven through a deck, because no repo deck
	// carries a ControlPlayer card and the redirect itself is rules-tested.
	m.e.G.ControlledBy = map[state.PlayerID]state.PlayerID{1: 0}
	m.e.G.ControlArmedTurn = map[state.PlayerID]int32{1: 0}

	brd := botpolicy.NewBoard(3)
	pd := projectNext(m, m.slots, &brd)
	if pd == nil {
		t.Fatalf("projectNext built no parked data: %+v", pd)
	}
	var pv1, pv2 *view.PlayerView
	for i := range pd.v.Players {
		switch pd.v.Players[i].ID {
		case 1:
			pv1 = &pd.v.Players[i]
		case 2:
			pv2 = &pd.v.Players[i]
		}
	}
	if pv1 == nil || pv2 == nil {
		t.Fatalf("expected seats 1 and 2: %+v", pd.v.Players)
	}
	if pv1.Hand == nil {
		t.Fatal("controller's view hides the controlled seat 1's hand (CR 723.4)")
	}
	if pv2.Hand != nil {
		t.Fatal("controller's view leaked uninvolved seat 2's hand")
	}
}

// TestControlledSeatsIsTheLiveActiveSeatOnly pins the host's widening set
// directly (cr-g57): at most one seat, the ACTIVE one, and only while its
// control is live (state.Game.LiveController: a turn strictly after the armed
// turn); only the viewer named as its controller gets it. Several grants can
// be folded at once (seats 1 and 3 controlled by 0, seat 2 by 1), but only
// the active seat's is live.
func TestControlledSeatsIsTheLiveActiveSeatOnly(t *testing.T) {
	t.Parallel()
	g := state.NewGame([]string{"a", "b", "c", "d"})
	g.ControlledBy = map[state.PlayerID]state.PlayerID{3: 0, 1: 0, 2: 1}
	g.ControlArmedTurn = map[state.PlayerID]int32{3: 1, 1: 1, 2: 1}
	g.Active, g.Turn = 0, 1
	if got := controlledSeats(g, 0); len(got) != 0 {
		t.Fatalf("controlledSeats(0) on seat 0's turn = %v, want none (no controlled seat is active)", got)
	}
	g.Active = 1
	if got := controlledSeats(g, 0); len(got) != 0 {
		t.Fatalf("controlledSeats(0) on the armed turn = %v, want none (CR 723.1: the NEXT turn)", got)
	}
	g.Turn = 2
	if got := controlledSeats(g, 0); len(got) != 1 || got[0] != 1 {
		t.Fatalf("controlledSeats(0) on seat 1's next turn = %v, want [1]", got)
	}
	if got := controlledSeats(g, 1); len(got) != 0 {
		t.Fatalf("controlledSeats(1) = %v on seat 1's own controlled turn, want none", got)
	}
	g.Active, g.Turn = 2, 3
	if got := controlledSeats(g, 1); len(got) != 1 || got[0] != 2 {
		t.Fatalf("controlledSeats(1) on seat 2's next turn = %v, want [2]", got)
	}
	if got := controlledSeats(g, 0); len(got) != 0 {
		t.Fatalf("controlledSeats(0) on seat 2's turn = %v, want none", got)
	}
}
