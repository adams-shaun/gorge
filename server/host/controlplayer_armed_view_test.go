package host

// cr-g57: a CR 723 grant is folded into state.Game.ControlledBy when it
// resolves, but the control is live only during the controlled player's NEXT
// turn (CR 723.1). projectNext's controlledSeats must not widen the
// controller's View (CR 723.4) with the controlled seat's hand before that
// turn begins: rules' controlPlayerRedirect hands the controller none of that
// seat's decisions yet, and the controller is answering its OWN decision.

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestProjectNextHidesAnArmedControlBeforeTheControlledTurn parks a live
// 3-seat table on human seat 0's own decision, folds a grant of seat 1 to
// seat 0 armed on the CURRENT turn (Turn == ControlArmedTurn: the grant
// waits for seat 1's NEXT turn whoever is active now), and projects through
// the real projectNext: seat 1's hand stays hidden, and controlledSeats names
// nobody.
func TestProjectNextHidesAnArmedControlBeforeTheControlledTurn(t *testing.T) {
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
	waitPending(t, r, "t1", 1, 0)
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	m := tb.cur
	if m == nil || m.e == nil || m.e.Pending() == nil || m.e.Pending().Player != 0 {
		t.Fatal("precondition: live match parked on seat 0's decision")
	}
	if len(m.e.G.Zone(state.ZHand, 1)) == 0 {
		t.Fatal("precondition: seat 1 has no hand to keep hidden")
	}
	m.e.G.ControlledBy = map[state.PlayerID]state.PlayerID{1: 0}
	m.e.G.ControlArmedTurn = map[state.PlayerID]int32{1: m.e.G.Turn}

	if got := controlledSeats(m.e.G, 0); len(got) != 0 {
		t.Errorf("controlledSeats(0) = %v before seat 1's controlled turn, want none", got)
	}
	brd := botpolicy.NewBoard(3)
	pd := projectNext(m, m.slots, &brd)
	if pd == nil {
		t.Fatal("projectNext built no parked data")
	}
	for _, pv := range pd.v.Players {
		if pv.ID == 1 && pv.Hand != nil {
			t.Fatalf("seat 0's own decision is projected with seat 1's %d-card hand before the controlled turn (CR 723.1/723.4)", len(pv.Hand))
		}
	}
}
