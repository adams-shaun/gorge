package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestLandsPlayedFilteredCount pins PlayerCountPropertyYou$LandsPlayed
// <filter>: the argument filters this turn's land plays by origin zone and
// ownership (Player.LandsPlayedOffHand records the non-hand / not-owned
// plays; the rest of LandsPlayed are owned hand plays). An unknown filter
// word fails closed.
func TestLandsPlayedFilteredCount(t *testing.T) {
	h, c := fixtureHost(t)
	c.Controller = 0
	const notHand = "PlayerCountPropertyYou$LandsPlayed Card.!inRealZoneHand,Card.YouDontOwn"
	const fromExile = "PlayerCountPropertyYou$LandsPlayed Card.inRealZoneExile"
	pl := &h.g.Players[0]

	// A land from hand: the unfiltered count is 1, the filtered one 0.
	pl.LandsPlayed, pl.LandsPlayedOffHand = 1, nil
	if got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$LandsPlayed"); !ok || got != 1 {
		t.Fatalf("precondition: unfiltered LandsPlayed = (%d, %v), want (1, true)", got, ok)
	}
	for _, tc := range []struct {
		name, expr string
		off        []state.LandPlay
		played     int32
		want       int32
	}{
		{"hand land", notHand, nil, 1, 0},
		{"graveyard land", notHand, []state.LandPlay{{From: state.ZGraveyard, Owner: 0}}, 1, 1},
		{"hand then graveyard", notHand, []state.LandPlay{{From: state.ZGraveyard, Owner: 0}}, 2, 1},
		{"opponent's card", notHand, []state.LandPlay{{From: state.ZHand, Owner: 1}}, 1, 1},
		{"exile filter on graveyard land", fromExile, []state.LandPlay{{From: state.ZGraveyard, Owner: 0}}, 1, 0},
		{"exile filter on exile land", fromExile, []state.LandPlay{{From: state.ZExile, Owner: 1}}, 2, 1},
		{"hand filter", "PlayerCountPropertyYou$LandsPlayed Card.inRealZoneHand+YouOwn", []state.LandPlay{{From: state.ZExile, Owner: 0}}, 3, 2},
	} {
		pl.LandsPlayed, pl.LandsPlayedOffHand = tc.played, tc.off
		if got, ok := EvalCountOK(h, c, tc.expr); !ok || got != tc.want {
			t.Errorf("%s: %s = (%d, %v), want (%d, true)", tc.name, tc.expr, got, ok, tc.want)
		}
	}

	pl.LandsPlayed, pl.LandsPlayedOffHand = 1, nil
	if _, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$LandsPlayed Card.Creature"); ok {
		t.Error("an unsupported filter word must fail closed")
	}
}
