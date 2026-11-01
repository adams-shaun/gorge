package event_test

import (
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// TestRoundRobinPairings pins the circle-method table literally and proves
// every pair meets exactly once.
func TestRoundRobinPairings(t *testing.T) {
	want := map[int][][2]int{
		1: {{0, 3}, {1, 2}},
		2: {{0, 2}, {1, 3}},
		3: {{0, 1}, {2, 3}},
	}
	for r := 1; r <= 3; r++ {
		got := event.Pairings(r)
		if len(got) != len(want[r]) {
			t.Fatalf("round %d has %d pairs, want %d", r, len(got), len(want[r]))
		}
		for i, p := range want[r] {
			if got[i] != p {
				t.Errorf("round %d pair %d is %v, want %v", r, i, got[i], p)
			}
		}
	}
	if got := event.Pairings(4); got != nil {
		t.Errorf("Pairings(4) = %v, want nil (only 3 rounds exist)", got)
	}

	seen := map[[2]int]int{}
	for r := 1; r <= 3; r++ {
		for _, p := range event.Pairings(r) {
			seen[p]++
		}
	}
	if len(seen) != 6 {
		t.Fatalf("the table holds %d distinct pairs, want 6", len(seen))
	}
	for p, n := range seen {
		if n != 1 {
			t.Errorf("pair %v meets %d times, want exactly once", p, n)
		}
	}
}
