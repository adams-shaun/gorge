package view_test

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestStartingChoiceHidesHands is the view half of CR 103.5: while the CR
// 103.1 play/draw choice is unresolved no player may see any hand. A seat's
// own projected Hand is a non-nil empty list (never the null another seat's
// hidden hand marshals) with the count still in HandSize, the
// opponent's Hand stays nil, and no hand object id is serialised anywhere in
// the seat view -- the hand, the library (its complement), and the
// PotentialActions offers that would carry hand card ids are all suppressed.
// The omniscient spectator view is unchanged: it still sees every hand.
func TestStartingChoiceHidesHands(t *testing.T) {
	e := rules.NewStartingPlayerChoice(rules.Config{
		Seed: 1, Names: []string{"a", "b"}, Decks: mountainDecks(t, 2),
	})
	if !e.StartingChoicePending() {
		t.Fatalf("fixture precondition: the CR 103.1 choice is not pending")
	}
	if e.G.Turn != 0 || e.G.Over {
		t.Fatalf("fixture precondition: turn=%d over=%v, want genesis", e.G.Turn, e.G.Over)
	}
	d := e.AskStartingPlayer()
	if d == nil {
		t.Fatalf("fixture precondition: no pending starting-player decision")
	}

	for _, viewer := range []state.PlayerID{0, 1} {
		v := view.ProjectFor(e.G, e, viewer, view.Seat, d)
		own := v.Players[viewer]
		if own.HandSize != 7 {
			t.Errorf("viewer %d: hand_size = %d, want 7 (the count is public)", viewer, own.HandSize)
		}
		if own.Hand == nil {
			t.Errorf("viewer %d: own Hand is nil, want a non-nil empty list", viewer)
		}
		if len(own.Hand) != 0 {
			t.Errorf("viewer %d: own Hand lists %d cards, want 0 while the choice is pending", viewer, len(own.Hand))
		}
		if other := v.Players[1-viewer]; other.Hand != nil {
			t.Errorf("viewer %d: opponent Hand = %v, want nil", viewer, other.Hand)
		}
		assertNoHandIDsSerialised(t, viewer, v, e.G.Zone(state.ZHand, viewer))
	}

	// Omniscient spectator: every hand still projected, unchanged by the
	// pending choice (CR 103.5 blinds players, not the omniscient view).
	omni := view.ProjectFor(e.G, e, view.NoSeat, view.Omniscient, d)
	for p := 0; p < len(omni.Players); p++ {
		if got := len(omni.Players[p].Hand); got != 7 {
			t.Errorf("omniscient seat %d: Hand lists %d cards, want 7", p, got)
		}
	}
}

// assertNoHandIDsSerialised marshals the view and walks the JSON for any
// object id equal to one of the hidden hand's ids: the projection's contract
// is about what reaches the wire, so the assertion reads the wire, not the
// in-process struct.
func assertNoHandIDsSerialised(t *testing.T, viewer state.PlayerID, v view.View, handIDs []state.ObjID) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("viewer %d: marshal view: %v", viewer, err)
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("viewer %d: unmarshal view: %v", viewer, err)
	}
	want := make(map[float64]bool, len(handIDs))
	for _, id := range handIDs {
		want[float64(id)] = true
	}
	var leaked []float64
	var walk func(any)
	walk = func(x any) {
		switch n := x.(type) {
		case map[string]any:
			if id, ok := n["id"].(float64); ok && want[id] {
				leaked = append(leaked, id)
			}
			for _, vv := range n {
				walk(vv)
			}
		case []any:
			for _, vv := range n {
				walk(vv)
			}
		}
	}
	walk(doc)
	if len(leaked) > 0 {
		t.Errorf("viewer %d: seat view serialised %d hidden hand object id(s): %v", viewer, len(leaked), leaked)
	}
}
