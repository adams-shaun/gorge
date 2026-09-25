package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestPlayerChosenRememberedQualifierChecksBase pins the review-found defect in
// matchesPlayerSingleSpec's Chosen/IsRemembered membership read: membership
// alone admitted a seat regardless of the qualified base, so a remembered
// controller matched `Opponent.IsRemembered` (Will the Wise's chain would then
// count its own seat among the opponents) and a remembered opponent matched
// `You.IsRemembered`. The base must gate the membership: every base the
// grammar evaluates (Player/Any always, You/Opponent/Other relative to the
// perspective seat) admits only seats that base actually names.
//
// Each subtest asserts BOTH directions — the positive base spelling admits
// the member (so the membership read still works) and the opposite base
// spelling excludes it (so the base is really checked). A test that only
// asserted the negative would pass with the membership read broken entirely.
func TestPlayerChosenRememberedQualifierChecksBase(t *testing.T) {
	src := state.NewGame(names(2))
	o := src.AddObject(creature(t, "Source"), 0)

	// mem remembers the player(s) named, in the source's own event-backed
	// player list — the home every base reads.
	mem := func(qualifier string, ps ...state.PlayerID) {
		o.Remembered = nil
		o.Chosen = nil
		for _, p := range ps {
			tgt := state.Target{Player: p, IsPlayer: true}
			if qualifier == "IsRemembered" {
				o.Remembered = append(o.Remembered, tgt)
			} else {
				o.Chosen = append(o.Chosen, tgt)
			}
		}
	}

	for _, tc := range []struct {
		qualifier  string
		member     state.PlayerID // the seat recorded on the source
		memberBase string         // base spelling that must admit member
		base       string         // base spelling under test
		you        state.PlayerID
		p          state.PlayerID
		want       bool
	}{
		// The controller remembers ITSELF: Opponent must not admit it.
		{"IsRemembered", 0, "You", "Opponent", 0, 0, false},
		{"IsRemembered", 0, "You", "You", 0, 0, true},
		{"IsRemembered", 0, "You", "Player", 0, 0, true},
		// The source remembers an opponent: You must not admit it.
		{"IsRemembered", 1, "Opponent", "You", 0, 1, false},
		{"IsRemembered", 1, "Opponent", "Opponent", 0, 1, true},
		// The same base check for Chosen (the sibling qualifier sharing the
		// membership read).
		{"Chosen", 0, "You", "Opponent", 0, 0, false},
		{"Chosen", 1, "Opponent", "You", 0, 1, false},
		{"Chosen", 1, "Opponent", "Opponent", 0, 1, true},
	} {
		mem(tc.qualifier, tc.member)
		spec := tc.base + "." + tc.qualifier
		got := MatchesPlayerSpecFrom(src, spec, tc.p, tc.you, o.ID)
		if got != tc.want {
			t.Errorf("%s for seat %d (you=%d, remembers %d) = %v, want %v",
				spec, tc.p, tc.you, tc.member, got, tc.want)
		}
	}
}
