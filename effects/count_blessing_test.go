// Count$Blessing.<yes>.<no> — CR 702.131's city's-blessing branch head (task
// activation-blessing-gate), the third read of the one-way blessing latch
// beside the bare Condition$ Blessing gate (effects/conditions.go) and the
// Activation$ Blessing offer gate (rules/legal.go's activationConditionOK).
// Unit-tested at the effects-side eval level the sibling branch heads use;
// the latch itself is the state.Player.Blessing bit events.Apply's
// BlessingChange fold writes, so this test drives it through h.Emit rather
// than writing the field directly.

package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestCountBlessingHeadSelectsBranchByTheLatch pins the corpus's five branch
// spellings (Count$Blessing.1.0 Golden Demise, .2.1 Anduril/Pride of
// Conquerors, .3.2 Secrets of the Golden City, .0.1 the "unless you have it"
// forks) against the real latch: the first token is the value when the
// resolving seat is blessed, the second when it is not.
func TestCountBlessingHeadSelectsBranchByTheLatch(t *testing.T) {
	h, c := fixtureHost(t)
	// Precondition: the fixture's seat 0 starts unblessed, or the false
	// branch below would be asserting on the wrong state.
	if h.g.Players[0].Blessing {
		t.Fatalf("fixture seat 0 already holds the blessing; the false branch is vacuous")
	}
	cases := []struct {
		expr      string
		blessed   int32
		unblessed int32
	}{
		{"Count$Blessing.1.0", 1, 0},
		{"Count$Blessing.2.1", 2, 1},
		{"Count$Blessing.3.2", 3, 2},
		{"Count$Blessing.0.1", 0, 1},
	}
	for _, tc := range cases {
		if got := EvalCount(h, c, tc.expr); got != tc.unblessed {
			t.Errorf("%s unblessed = %d, want %d", tc.expr, got, tc.unblessed)
		}
		h.Emit(events.Event{Kind: events.BlessingChange, Player: 0, Text: "city's blessing"})
		if !h.g.Players[0].Blessing {
			t.Fatalf("BlessingChange did not latch seat 0's blessing")
		}
		if got := EvalCount(h, c, tc.expr); got != tc.blessed {
			t.Errorf("%s blessed = %d, want %d", tc.expr, got, tc.blessed)
		}
		// Reset the latch for the next case: the fold is one-way in play,
		// but each case must start from the unblessed state to be a real
		// branch test. A plain field write here is fine — this is the
		// effects-package double, not engine state.
		h.g.Players[0].Blessing = false
	}
}

// TestCountBlessingHeadReadsTheResolvingSeatsLatch pins the CONTROLLER
// scoping: the head reads the resolving ability's controller, not the active
// player or the source's arbitrary owner. Seat 1 held the blessing must not
// answer for a seat-0 resolving ability.
func TestCountBlessingHeadReadsTheResolvingSeatsLatch(t *testing.T) {
	h, c := fixtureHost(t)
	h.Emit(events.Event{Kind: events.BlessingChange, Player: 1, Text: "city's blessing"})
	if !h.g.Players[1].Blessing {
		t.Fatalf("BlessingChange did not latch seat 1's blessing")
	}
	if h.g.Players[0].Blessing {
		t.Fatalf("seat 0 latched with seat 1's blessing event; the scoping test is vacuous")
	}
	if got := EvalCount(h, c, "Count$Blessing.1.0"); got != 0 {
		t.Errorf("seat 0 resolving with only seat 1 blessed = %d, want the false branch's 0", got)
	}
	c.Controller = 1
	if got := EvalCount(h, c, "Count$Blessing.1.0"); got != 1 {
		t.Errorf("seat 1 resolving blessed = %d, want the true branch's 1", got)
	}
	// Out-of-range controller denies (the sibling heads' fail-closed read).
	c.Controller = state.PlayerID(99)
	if got := EvalCount(h, c, "Count$Blessing.1.0"); got != 0 {
		t.Errorf("out-of-range controller = %d, want the false branch's 0", got)
	}
}
