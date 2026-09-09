package host

import (
	"testing"

	"github.com/adams-shaun/gorge/protocol"
)

// TestBoundsOfMatchesTheLoopsOwnBookkeepingWithAMidResolutionAsk is the
// coverage the flat TestBoundsOfMatchesTheLoopsOwnBookkeeping (viewat_test.go)
// could not provide: fourSeatTable (seed 99) has no mid-resolution ask, so its
// stream never puts a DecisionAsk and a Priority in the same burst, and the
// derivation is never exercised against the suspended path — the shape that
// used to make boundsOf fall one event early (rules/legal.go's pass-branch
// emit and rules/resolution.go's completion grant). The modal fixture's game
// does contain a mid-resolution "modes" ask, and this leaf requires that the
// derived bounds equal the loop's own recorded bookkeeping for it. It fails on
// the old pass-branch emit (which logged a Priority between the
// mid-resolution DecisionAsk and its answer), so it is the host-side
// regression guard for the fix.
func TestBoundsOfMatchesTheLoopsOwnBookkeepingWithAMidResolutionAsk(t *testing.T) {
	t.Parallel()
	m := modalFinishedTable(t)
	if !matchHasDecisionAsk(m.e.L.Events, "modes") {
		t.Fatal("mid-resolution fixture produced no 'modes' ask — the leaf proves nothing")
	}
	got := boundsOf(m.e.L.Events)
	if len(got) != len(m.bounds) {
		t.Fatalf("boundsOf found %d boundaries, the loop recorded %d", len(got), len(m.bounds))
	}
	for i := range got {
		if got[i] != m.bounds[i] {
			t.Fatalf("boundary %d: derived %d, recorded %d", i, got[i], m.bounds[i])
		}
	}
	if m.state != protocol.MatchFinished {
		t.Fatalf("mid-resolution fixture did not finish: %s", m.state)
	}
}
