package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
)

// TestInvariantsUnderSeedFuzz is the rules core's acceptance gate: many games,
// every one terminating, invariants intact throughout. This is the test that
// found the stack double-push bug in the design spike.
func TestInvariantsUnderSeedFuzz(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	names, decks := testutil.SampleDecks(t, 4)
	finished, totalEvents := 0, 0
	for seed := uint64(0); seed < 60; seed++ {
		e := New(Config{Seed: seed, Names: names, Decks: decks})
		// Ruling P6: e.L.NoHash = true here would panic -- events.Log.NoHash
		// is immutable after the first Append, and New already appended
		// GameStart before returning this Engine. Hashing every event is
		// cheap enough that there is nothing worth trading it away for.
		b := newTestBot(seed * 31)
		e.Advance()
		testutil.CheckInvariants(t, e.G, e.Pending(), "start")
		n := 0
		for !e.G.Over && e.Pending() != nil && n < 200000 {
			if err := e.Submit(b.answer(e.Pending())); err != nil {
				t.Fatalf("seed %d intent %d: %v", seed, n, err)
			}
			if n%97 == 0 {
				testutil.CheckInvariants(t, e.G, e.Pending(), "mid")
			}
			n++
		}
		testutil.CheckInvariants(t, e.G, e.Pending(), "end")
		if !e.G.Over {
			t.Errorf("seed %d did not terminate after %d intents (turn %d)", seed, n, e.G.Turn)
			continue
		}
		finished++
		totalEvents += len(e.L.Events)
	}
	if finished != 60 {
		t.Fatalf("%d of 60 seeds finished", finished)
	}
	t.Logf("60 seeds finished, %d events, invariants held", totalEvents)
}
