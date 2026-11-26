package codeshape

import (
	"strings"
	"testing"
)

// TestRatchetLoweringOnlyPostMerge holds the guard that keeps a pull request from lowering a
// shrink-only ratchet constant: a lower constant is refused before merge, so a batch cannot
// combine it with a branch that grew the same measurement back to the base's value.
func TestRatchetLoweringOnlyPostMerge(t *testing.T) {
	baseConsts := []byte("package codeshape\n\nconst (\n\tmaxFuncLinesOver300 = 400\n\tengineMethodCount = 100\n)\n")
	lowerConsts := []byte("package codeshape\n\nconst (\n\tmaxFuncLinesOver300 = 399\n\tengineMethodCount = 100\n)\n")
	got, err := LoweredConstants(baseConsts, lowerConsts)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "maxFuncLinesOver300") {
		t.Fatalf("lowered constants = %q, want maxFuncLinesOver300", got)
	}
	report := LoweringReport(got)
	if !strings.Contains(report, LoweringPostMergeOnly) {
		t.Fatalf("report %q does not say %q", report, LoweringPostMergeOnly)
	}
	for _, tc := range []struct {
		name string
		head []byte
		want int
	}{
		{"raised", []byte("package codeshape\n\nconst (\n\tmaxFuncLinesOver300 = 401\n\tengineMethodCount = 100\n)\n"), 0},
		{"held", []byte("package codeshape\n\nconst (\n\tmaxFuncLinesOver300 = 400\n\tengineMethodCount = 100\n)\n"), 0},
		{"removed", []byte("package codeshape\n\nconst (\n\tengineMethodCount = 100\n)\n"), 1},
	} {
		got, err := LoweredConstants(baseConsts, tc.head)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(got) != tc.want {
			t.Errorf("%s: lowered = %q, want %d entries", tc.name, got, tc.want)
		}
	}
}
