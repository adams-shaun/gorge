package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestPlayerCountPropertyYouPerTurnLedgerCounts(t *testing.T) {
	h, c := fixtureHost(t)
	c.Controller = 0
	h.g.Players[0].LandsPlayed = 2
	h.lifeLost = map[state.PlayerID]int32{0: 3, 1: 7}
	h.discarded = map[state.PlayerID]int32{0: 2, 1: 4}

	cases := []struct {
		property string
		want     int32
	}{
		{"CardsDiscardedThisTurn", 2},
		{"LifeLostThisTurn", 3},
		{"LandsPlayed", 2},
	}
	for _, tc := range cases {
		got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$"+tc.property)
		if !ok || got != tc.want {
			t.Errorf("%s = (%d, %v), want (%d, true)", tc.property, got, ok, tc.want)
		}
	}

	// Changing the resolving controller must read that player's ledger, not
	// the source controller's values.
	c.Controller = 1
	for _, tc := range []struct {
		property string
		want     int32
	}{
		{"CardsDiscardedThisTurn", 4},
		{"LifeLostThisTurn", 7},
		{"LandsPlayed", 0},
	} {
		got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$"+tc.property)
		if !ok || got != tc.want {
			t.Errorf("controller 1 %s = (%d, %v), want (%d, true)", tc.property, got, ok, tc.want)
		}
	}

	// The sacrifice event stream has no actor provenance. Do not resolve this
	// count from the permanent's owner or controller; either can differ from
	// the player who performed the sacrifice.
	if got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$SacrificedThisTurn"); ok || got != 0 {
		t.Errorf("SacrificedThisTurn = (%d, %v), want unresolved (0, false) without actor provenance", got, ok)
	}
}
