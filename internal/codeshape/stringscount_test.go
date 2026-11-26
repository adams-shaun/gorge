package codeshape

import (
	"path/filepath"
	"testing"
)

// TestStringAllowlistStaysFresh refuses an allowlist entry no scope holds
// any more: a boundary or reporting file that moved or was retired leaves a
// stale entry whose skip is dead (and whose reason is misleading).
func TestStringAllowlistStaysFresh(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	seen := map[string]bool{}
	for _, scope := range StringScopes {
		if _, err := MeasureStrings(root, scope, seen); err != nil {
			t.Fatalf("measure %s: %v", scope, err)
		}
	}
	for f := range StringAllowlist {
		if !seen[f] {
			t.Errorf("StringAllowlist names %s, which no scope holds: delete the stale entry", f)
		}
	}
}
