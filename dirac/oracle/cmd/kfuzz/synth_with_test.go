package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFuzzSynthWithFlag: -synth-with is a usage error without -synth or -focus, and the
// evloop case plays: Adarkar Windform (tail-00, a {S} cost) beside the Snow-Covered
// Islands tail-52 holds, with a coverage row for the focus card.
func TestFuzzSynthWithFlag(t *testing.T) {
	cardsettest.Synth(t, "tail-00")
	cardsettest.Synth(t, "tail-52")
	// run() stores -coverage in package globals; a later test's report() would write to
	// this test's removed temp dir and exit 1.
	oldCov, oldEv, oldBr := coveragePath, evidencePath, branchesPath
	t.Cleanup(func() { coveragePath, evidencePath, branchesPath = oldCov, oldEv, oldBr })
	dir := t.TempDir()
	focus := filepath.Join(dir, "f.txt")
	only := filepath.Join(dir, "o.txt")
	if err := os.WriteFile(focus, []byte("Adarkar Windform\tSnow-Covered Island;Snow-Covered Island\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(only, []byte("Adarkar Windform\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-synth-with", "tail-52"},
		{"-synth-with", "tail-52", "-focus", focus},
		{"-synth", "tail-00", "-synth-with", "tail-52"},
		{"-synth", "tail-00", "-synth-with", "", "-focus", focus},
	} {
		if code, _, stderr := runKFuzz(t, args...); code != 2 || !strings.Contains(stderr, "-synth-with") {
			t.Errorf("%v: exit %d, stderr %q: want exit 2 naming -synth-with", args, code, stderr)
		}
	}

	base := []string{"-synth", "tail-00", "-focus", focus, "-focus-per-deck", "1", "-only", only, "-pairs", "4", "-seed", "1", "-surface", "full", "-shrink", "0"}
	if code, _, stderr := runKFuzz(t, base...); code == 0 || !strings.Contains(stderr, `enabler "Snow-Covered Island" of "Adarkar Windform" is not a card tail-00 plays`) {
		t.Errorf("without -synth-with: exit %d, stderr %q: want the enabler error", code, stderr)
	}
	cov := filepath.Join(dir, "cov.tsv")
	code, stdout, stderr := runKFuzz(t, append(base, "-synth-with", "tail-52", "-coverage", cov)...)
	if code > 1 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	// The run's name is the focus file's base name. Its game count is not pinned: a pair can skip as unsupported when
	// the corpus or kgen's supported set moves (11-20: batches #2803/#2804 saw "f: 3 games (1 skipped as unsupported)").
	if !regexp.MustCompile(`(?m)^f: [1-4] games \(`).MatchString(stdout) {
		t.Errorf("the run is not named after the focus file:\n%s", stdout)
	}
	b, err := os.ReadFile(cov)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains("\n"+string(b), "\nAdarkar Windform\t") {
		t.Errorf("no coverage row for the focus card:\n%.400s", b)
	}
	if !strings.Contains("\n"+string(b), "\nSnow-Covered Island\t") {
		t.Errorf("no coverage row for the lent enabler: it was not dealt:\n%.400s", b)
	}
}
