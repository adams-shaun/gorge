package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPlayerCountRememberedCounters pins PlayerCountRemembered$Counters.<KIND>
// (Phyresis Outbreak's poison-count X value) against the remembered players
// only: the resolving controller's own poison counters must not be summed in.
func TestPlayerCountRememberedCounters(t *testing.T) {
	g := state.NewGame(names(4))
	h := &fakeHost{g: g}
	h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: 1, Counter: "POISON", Amount: 2})
	h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: 3, Counter: "POISON", Amount: 3})
	h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "POISON", Amount: 7})

	// Preconditions the real assertion depends on: the counters are where the
	// rule reads them, and the remembered total (2+3) differs from both the
	// controller's 7 and the full table total (12).
	if got := g.Players[1].Counter("POISON"); got != 2 {
		t.Fatalf("fixture: player 1 poison = %d, want 2", got)
	}
	if got := g.Players[3].Counter("POISON"); got != 3 {
		t.Fatalf("fixture: player 3 poison = %d, want 3", got)
	}
	if got := g.Players[0].Counter("POISON"); got != 7 {
		t.Fatalf("fixture: controller poison = %d, want 7", got)
	}

	c := &Ctx{Controller: 0, Remembered: []state.Target{
		{Player: 1, IsPlayer: true},
		{Player: 3, IsPlayer: true},
	}}
	for _, tc := range []struct {
		expr string
		want int32
	}{
		{"Count$PlayerCountRemembered$Counters.Poison", 5},
		{"Count$PlayerCountRemembered$Counters.ALL", 5},
	} {
		got, ok := EvalCountOK(h, c, tc.expr)
		if !ok {
			t.Fatalf("EvalCountOK(%q) reported UNRESOLVED, want (%d,true)", tc.expr, tc.want)
		}
		if got != tc.want {
			t.Fatalf("EvalCountOK(%q)=(%d,%v), want (%d,true): the remembered players' 2+3, "+
				"not the controller's 7 or the table's 12", tc.expr, got, ok, tc.want)
		}
	}
}
