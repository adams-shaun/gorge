package codeshape

import (
	"strings"
	"testing"
)

// TestRatchetLoweringOnlyPostMerge holds the guard that keeps a pull request from lowering a
// shrink-only ratchet: a lower row or constant is refused before merge, so a batch cannot
// combine it with a branch that grew the same function back to the base's row.
func TestRatchetLoweringOnlyPostMerge(t *testing.T) {
	baseCeilings := []byte("# key\tlines\tcomplexity\np big\t400\t70\np small\t299\t10\n")
	lowerCeilings := []byte("# key\tlines\tcomplexity\np big\t399\t70\np small\t299\t10\n")
	growCeilings := []byte("# key\tlines\tcomplexity\np big\t400\t70\np small\t299\t10\n")

	lowered, err := LoweredCeilings(baseCeilings, lowerCeilings)
	if err != nil {
		t.Fatal(err)
	}
	if len(lowered) != 1 || !strings.Contains(lowered[0], "p big") {
		t.Fatalf("lowered ceilings = %q, want the p big row", lowered)
	}
	report := LoweringReport(lowered)
	if !strings.Contains(report, LoweringPostMergeOnly) {
		t.Fatalf("report %q does not say %q", report, LoweringPostMergeOnly)
	}

	unchanged, err := LoweredCeilings(baseCeilings, growCeilings)
	if err != nil {
		t.Fatal(err)
	}
	if len(unchanged) != 0 {
		t.Fatalf("a row held at its base value was reported as lowered: %q", unchanged)
	}

	// Acceptance: the growing branch is green against the base's row, and would only be red
	// if the lowering branch had merged first. The guard keeps that branch out of the batch.
	// (A lowering of one line sits inside FuncSlack, so the fixture grows to the base's row plus its
	// slack: green against the base's row, one line too many against the lowered one.)
	shapes := []FuncShape{{Name: "big", File: "p/f.go", Line: 1, Lines: 400 + FuncSlack(400), Complexity: 70}}
	baseRows := mustCeilings(t, baseCeilings)
	if grew, _, _ := funcShapeCheck(shapes, baseRows, linesMetric); len(grew) != 0 {
		t.Fatalf("growth to the base's row fails without the lowering merged: %q", grew)
	}
	lowerRows := mustCeilings(t, lowerCeilings)
	if grew, _, _ := funcShapeCheck(shapes, lowerRows, linesMetric); len(grew) != 1 {
		t.Fatalf("growth past the lowered row did not fail, so the fixture proves nothing: %q", grew)
	}

	// A removed row, or a rename that does not keep the values, is a lowering; a rename with
	// the same values is the move invariant 8 allows.
	for _, tc := range []struct {
		name string
		head []byte
		want int
	}{
		{"removed", []byte("# key\tlines\tcomplexity\np small\t299\t10\n"), 1},
		{"renamed with the same ceiling", []byte("# key\tlines\tcomplexity\np renamed\t400\t70\np small\t299\t10\n"), 0},
		{"renamed lower", []byte("# key\tlines\tcomplexity\np renamed\t399\t70\np small\t299\t10\n"), 1},
	} {
		got, err := LoweredCeilings(baseCeilings, tc.head)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(got) != tc.want {
			t.Errorf("%s: lowered = %q, want %d entries", tc.name, got, tc.want)
		}
	}

	baseConsts := []byte("package codeshape\n\nconst (\n\tmaxFuncLinesOver300 = 400\n\tengineMethodCount = 100\n)\n")
	lowerConsts := []byte("package codeshape\n\nconst (\n\tmaxFuncLinesOver300 = 399\n\tengineMethodCount = 100\n)\n")
	got, err := LoweredConstants(baseConsts, lowerConsts)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "maxFuncLinesOver300") {
		t.Fatalf("lowered constants = %q, want maxFuncLinesOver300", got)
	}
	for _, tc := range []struct {
		name string
		head []byte
		want int
	}{
		{"raised", []byte("package codeshape\n\nconst (\n\tmaxFuncLinesOver300 = 401\n\tengineMethodCount = 100\n)\n"), 0},
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

func mustCeilings(t *testing.T, b []byte) map[string]funcCeiling {
	t.Helper()
	_, rows, err := parseFuncCeilings(b)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
