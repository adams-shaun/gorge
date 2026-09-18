package view

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// potentialChars wraps flatChars and lets a test drive a chosen
// PotentialActions per player, so the projection test can assert the gating
// (viewer's own seat only) without re-implementing the walk.
type potentialChars struct {
	flatChars
	pot func(state.PlayerID) []decision.PotentialAction
}

func (c potentialChars) PotentialActions(p state.PlayerID) []decision.PotentialAction {
	return c.pot(p)
}

// TestPotentialActionsGatedToViewerSeat pins the hidden-info gate on the
// projection (rv2c): the potential-action walk reads the seat's own hand,
// command zone and graveyard — a CR 400.2 hidden zone — so the wire carries
// the field for the VIEWER'S OWN SEAT ONLY. Every other seat's PlayerView is
// absent the key entirely (omitempty), whatever their boards hold, and a
// spectator (viewer naming no real seat) sees none.
func TestPotentialActionsGatedToViewerSeat(t *testing.T) {
	g := fourSeatBoard(t)
	ch := potentialChars{flatChars{g}, func(p state.PlayerID) []decision.PotentialAction {
		return []decision.PotentialAction{{Kind: "cast", Obj: 99, Label: "Cast something"}}
	}}

	// Viewer 0: seat 0 (their own seat) carries the field, seats 1..3 do not.
	v := ProjectFor(g, ch, 0, Seat, nil)
	if len(v.Players[0].PotentialActions) != 1 || v.Players[0].PotentialActions[0].Obj != 99 {
		t.Fatalf("viewer 0's own seat must carry their potential actions, got %+v", v.Players[0].PotentialActions)
	}
	for _, seat := range []state.PlayerID{1, 2, 3} {
		if v.Players[seat].PotentialActions != nil {
			t.Errorf("seat %d's PlayerView carries potential_actions for viewer 0 — a hidden-zone leak (CR 400.2)", seat)
		}
	}

	// Viewer 1: the same walk answers seat 1 only.
	v1 := ProjectFor(g, ch, 1, Seat, nil)
	if len(v1.Players[1].PotentialActions) != 1 {
		t.Fatal("viewer 1's own seat must carry their potential actions")
	}
	for _, seat := range []state.PlayerID{0, 2, 3} {
		if v1.Players[seat].PotentialActions != nil {
			t.Errorf("seat %d's PlayerView carries potential_actions for viewer 1", seat)
		}
	}

	// A spectator (NoSeat) never matches the viewer gate.
	vs := ProjectFor(g, ch, NoSeat, Public, nil)
	for _, p := range vs.Players {
		if p.PotentialActions != nil {
			t.Errorf("spectator view seat %d carries potential_actions", p.ID)
		}
	}
}
