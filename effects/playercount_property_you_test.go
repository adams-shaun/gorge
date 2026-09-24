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

	// SacrificedThisTurn reads the per-turn zone-entry record's sacrifice
	// stamp (events.Apply's MoveZone fold records the SACRIFICER -- the
	// permanent's controller at the move, CR 701.21a -- not its owner), so
	// the count is the resolving controller's own sacrifices only.
	var sacked state.ObjID
	for i := range h.g.Objs {
		if h.g.Objs[i].Face() != nil {
			sacked = h.g.Objs[i].ID
			break
		}
	}
	h.g.Entered = append(h.g.Entered, state.ZoneEntry{Obj: sacked, From: state.ZBattlefield,
		To: state.ZGraveyard, Sacrificed: true, Sacrificer: 1})
	if got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$SacrificedThisTurn"); !ok || got != 1 {
		t.Errorf("controller 1 SacrificedThisTurn = (%d, %v), want (1, true)", got, ok)
	}
	c.Controller = 0
	if got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$SacrificedThisTurn"); !ok || got != 0 {
		t.Errorf("controller 0 SacrificedThisTurn = (%d, %v), want (0, true): seat 1 sacrificed it", got, ok)
	}
}
