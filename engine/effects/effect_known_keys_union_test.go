package effects

import (
	"slices"
	"testing"
)

// TestEffectKnownKeysNormalizesTheUnion pins the merge=union repair for
// engine/effects/*_params.go: a rebase can leave a raw row with a duplicate or
// an out-of-order key, and the lookup table must still be the sorted union.
func TestEffectKnownKeysNormalizesTheUnion(t *testing.T) {
	t.Parallel()
	raw := []string{"B", "A", "B", "C"}
	if slices.Equal(raw, normalizeKnownKeys(raw)) {
		t.Fatalf("fixture is already normalized: %v", raw)
	}
	if got := normalizeKnownKeys(raw); !slices.Equal(got, []string{"A", "B", "C"}) {
		t.Fatalf("normalizeKnownKeys(%v) = %v, want [A B C]", raw, got)
	}
	for _, k := range effectKnownKeyRows {
		if !slices.Contains(effectKnownKeys, k) {
			t.Fatalf("normalized table dropped %q", k)
		}
	}
	if !slices.IsSorted(effectKnownKeys) ||
		len(slices.Compact(slices.Clone(effectKnownKeys))) != len(effectKnownKeys) {
		t.Fatalf("effectKnownKeys is not the sorted union: %v", effectKnownKeys)
	}
}
