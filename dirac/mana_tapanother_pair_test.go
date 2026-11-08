package dirac

import "testing"

// TestTapXPairElection pins the single pair walk both two-permanent tap
// searches use (tapXSearch.visit and filtSearch.otherGap): gorge's
// scriptAnswers enumerates every pair of a Min == Max == 2 object election in
// lexicographic order, capped at 64 answers. The four lockstep repros are the
// behavioural proof; this is the order's unit test.
func TestTapXPairElection(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want [][2]int
	}{
		{2, [][2]int{{0, 1}}},
		{4, [][2]int{{0, 1}, {0, 2}, {0, 3}, {1, 2}, {1, 3}, {2, 3}}},
		{5, [][2]int{
			{0, 1}, {0, 2}, {0, 3}, {0, 4},
			{1, 2}, {1, 3}, {1, 4},
			{2, 3}, {2, 4},
			{3, 4},
		}},
	} {
		if got := tapXPairLimit(tc.n); got != len(tc.want) {
			t.Fatalf("n=%d: limit %d, want %d (the old per-unit layout answered none of an odd count)",
				tc.n, got, len(tc.want))
		}
		for qi, w := range tc.want {
			i, j := tapXPairAt(tc.n, qi)
			if i != w[0] || j != w[1] {
				t.Fatalf("n=%d pair %d: got (%d,%d), want (%d,%d)", tc.n, qi, i, j, w[0], w[1])
			}
		}
	}

	// 13 candidates are 78 pairs, so the walk stops at the 64-answer cap.
	const n = 13
	if total := n * (n - 1) / 2; total <= 64 {
		t.Fatalf("precondition: C(%d,2) = %d must exceed the 64-answer cap", n, total)
	}
	var want [][2]int
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			want = append(want, [2]int{i, j})
		}
	}
	if len(want) != 78 {
		t.Fatalf("precondition: C(%d,2) generated %d pairs, want 78", n, len(want))
	}
	if want[63] != [2]int{7, 8} {
		t.Fatalf("precondition: pair 63 is %v, want (7,8)", want[63])
	}
	if got := tapXPairLimit(n); got != 64 {
		t.Fatalf("n=%d: limit %d, want the 64-answer cap", n, got)
	}
	for qi := 0; qi < 64; qi++ {
		i, j := tapXPairAt(n, qi)
		if i != want[qi][0] || j != want[qi][1] {
			t.Fatalf("n=%d pair %d: got (%d,%d), want (%d,%d)", n, qi, i, j, want[qi][0], want[qi][1])
		}
	}
}
