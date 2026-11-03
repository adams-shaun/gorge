package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFuzzSynthRuns plays a dealt pool from memory: the verdict and the reach line
// print, and two runs print the same.
func TestFuzzSynthRuns(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-fdn")
	reach := filepath.Join(t.TempDir(), "reach.tsv")
	args := []string{"-synth", "std-fdn", "-play-refused", "-first-pair", "0", "-pairs", "12", "-seed", "1", "-keep-going", "-surface", "autopay", "-shrink", "0", "-workers", "2", "-reach", reach}
	code, stdout, stderr := runKFuzz(t, args...)
	if code > 1 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{"std-fdn: 12 games (", "reach std-fdn: ", "cards in the list"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	b, err := os.ReadFile(reach)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "card\tstatus\tdecks\tgames\tplays\n") {
		t.Errorf("-reach: %.80q", b)
	}
	_, again, _ := runKFuzz(t, args...)
	if again != stdout {
		t.Errorf("two runs differ:\n%s\n---\n%s", stdout, again)
	}
}

func TestFuzzSynthFlags(t *testing.T) {
	for _, args := range [][]string{
		{"-synth", "std-dsk", "-kernel", "blb"},
		{"-synth", "std-dsk", "-set", "fdn"},
		{"-synth", "std-dsk", "-decks", "/x"},
		{"-synth-decks", "10"},
		{"-reach", "/x"},
	} {
		if code, _, stderr := runKFuzz(t, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2 (stderr %q)", args, code, stderr)
		}
	}
	cardsettest.Synth(t, "std-dsk")
	if code, _, stderr := runKFuzz(t, "-synth", "no-such-pool"); code != 2 || !strings.Contains(stderr, "no-such-pool") {
		t.Errorf("unknown pool: exit %d, stderr %q", code, stderr)
	}
}

// TestFuzzAutopayXCastPaysThroughTheWindow replays the std-sos pair whose
// Traumatic Critique, offered because the floating restricted {U}{R} pays its
// pips at X = 0, is cast with X = 6. gorge's planner prices no {X} spell, so
// the seat's cast pays the six through the manual window in battlefield order;
// dirac used to tap a witness plan (three Mountains, three Islands).
func TestFuzzAutopayXCastPaysThroughTheWindow(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-sos")
	code, stdout, stderr := runKFuzz(t, "-synth", "std-sos", "-first-pair", "1104", "-pairs", "1", "-seed", "1", "-surface", "autopay", "-shrink", "0", "-absolute")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q\n%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "1 clean, 0 divergent") {
		t.Errorf("pair 1104 diverged:\n%s", stdout)
	}
}
