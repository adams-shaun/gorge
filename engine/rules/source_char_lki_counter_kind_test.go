package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestSourceCharLKICounterKindFold: a departed source's snapshot answers
// Count$CardCounters.<KIND> through the same counter-kind fold as the live
// readers, so Wilfred Mott's "Count$CardCounters.Time" reads the TIME counters
// its own PutCounter placed even when it left the battlefield before its
// upkeep dig resolves (Forge reads LKI; CounterEnumType is case-insensitive).
func TestSourceCharLKICounterKindFold(t *testing.T) {
	snap := sourceCharSnapshot{counters: []state.Counter{{Kind: "TIME", N: 2}}}
	if n, ok := snap.count("Count$CardCounters.TIME"); !ok || n != 2 {
		t.Fatalf("precondition: Count$CardCounters.TIME = %d, %v; want 2, true", n, ok)
	}
	if n, ok := snap.count("Count$CardCounters.Time"); !ok || n != 2 {
		t.Fatalf("Count$CardCounters.Time = %d, %v; want 2, true", n, ok)
	}
}
