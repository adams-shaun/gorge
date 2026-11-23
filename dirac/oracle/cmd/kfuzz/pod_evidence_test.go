package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFuzzPodEvidence: kfuzz -evidence with -seats above 2 writes the run's
// evidence rows with a trailing seats field (Name\t<seats>), the column a land
// step records a card's clean pod games from; a two-seat run's rows stay bare
// names, one a line.
func TestFuzzPodEvidence(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-fdn")
	ev := filepath.Join(t.TempDir(), "pod.evidence")
	code, stdout, stderr := runKFuzz(t, "-synth", "std-fdn", "-seats", "4", "-pods", "3",
		"-seed", "1", "-keep-going", "-shrink", "0", "-workers", "2", "-evidence", ev)
	if code > 1 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	rows := evidenceRows(t, ev)
	if len(rows) == 0 {
		t.Fatalf("no pod evidence rows:\n%s", stdout)
	}
	for _, r := range rows {
		if !strings.HasSuffix(r, "\t4") {
			t.Errorf("row %q has no seats 4 field", r)
		}
	}
	if !podGamesPlayed(stdout) {
		t.Errorf("the pod verdict line is missing or plays no game:\n%s", stdout)
	}

	// A two-seat run writes the same rows without the field.
	ev2 := filepath.Join(t.TempDir(), "two.evidence")
	code, _, stderr = runKFuzz(t, "-synth", "std-fdn", "-pairs", "2", "-seed", "1",
		"-keep-going", "-shrink", "0", "-workers", "2", "-evidence", ev2)
	if code > 1 {
		t.Fatalf("two-seat exit %d, stderr %q", code, stderr)
	}
	for _, r := range evidenceRows(t, ev2) {
		if strings.Contains(r, "\t") {
			t.Errorf("a two-seat row carries a field: %q", r)
		}
	}
}

// podGamesPlayed reports the pods census line's played count: a pair the
// table refuses (an as-enters opponent choice beside another object's enters
// replacement, Spyglass beside Authority of the Consuls) is skipped, so the
// played games may sit under the posed pods.
func podGamesPlayed(stdout string) bool {
	i := strings.Index(stdout, " pods (4 seats): ")
	if i < 0 {
		return false
	}
	rest := stdout[i+len(" pods (4 seats): "):]
	n := rest[:strings.IndexByte(rest, ' ')]
	return n != "0"
}

// evidenceRows reads an evidence file's nonempty lines.
func evidenceRows(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, l := range strings.Split(string(b), "\n") {
		if l != "" {
			rows = append(rows, l)
		}
	}
	return rows
}
