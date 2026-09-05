package host

import "testing"

func TestMatchSeedIsAPureFunctionOfTableSeedAndIndex(t *testing.T) {
	if MatchSeed(1, 1) != MatchSeed(1, 1) {
		t.Fatal("not deterministic")
	}
	seen := map[uint64]bool{}
	for k := 1; k <= 1000; k++ {
		s := MatchSeed(42, k)
		if seen[s] {
			t.Fatalf("seed collision at k=%d", k)
		}
		seen[s] = true
	}
	if MatchSeed(1, 1) == MatchSeed(2, 1) || MatchSeed(1, 1) == MatchSeed(1, 2) {
		t.Fatal("seed does not depend on both inputs")
	}
	// Pinned so a table's history cannot silently change under a refactor.
	if got := MatchSeed(0, 1); got != 0xe220a8397b1dcdaf {
		t.Fatalf("MatchSeed(0,1) = %#x; if the formula changed on purpose, update this pin and the sidecar goldens", got)
	}
}
