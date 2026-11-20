package codeshape

import (
	"strings"
	"testing"
)

// TestLoweredCeilingsCoversADeleteAndRename: a function deleted with its row,
// and another renamed under a new key at its own ceiling, are changes invariant
// 8 allows (the shape ratchet still refuses an unlisted function over a
// threshold). Matching removed keys to new ones one-to-one in key order lets
// the deleted key claim the renamed key's row and reports the legal rename as a
// removal.
func TestLoweredCeilingsCoversADeleteAndRename(t *testing.T) {
	base := []byte("# key\tlines\tcomplexity\npA\t300\t10\npB\t400\t70\n")
	head := []byte("# key\tlines\tcomplexity\npX\t400\t70\n")
	got, err := LoweredCeilings(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("delete pA + rename pB->pX: lowered %q, want none", got)
	}
}

// TestLoweredCeilingsRefusesTheRenameThatLowersTheCoveredRow: the delete+rename
// allowance must not hide a rename below its old ceiling. pX covers pA but not
// pB, so pB's row is the one that was lowered and it is the one reported.
func TestLoweredCeilingsRefusesTheRenameThatLowersTheCoveredRow(t *testing.T) {
	base := []byte("# key\tlines\tcomplexity\npA\t300\t10\npB\t400\t70\n")
	head := []byte("# key\tlines\tcomplexity\npX\t399\t70\n")
	got, err := LoweredCeilings(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "pB") {
		t.Fatalf("rename pB below its old ceiling: lowered %q, want one entry for pB", got)
	}
}
