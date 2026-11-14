package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// TestSearchSeatsFlags: -search-seats is for pool runs with gorge leading;
// any other kernel, -lead kernel, -repro or a bad seat list exits 2 with a
// one-line reason, and -search or -search-sims alone are refused.
func TestSearchSeatsFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"gorge kernel", []string{"-kernel", "gorge", "-search-seats", "0"}, "pool runs"},
		{"vanilla kernel", []string{"-kernel", "vanilla", "-search-seats", "0"}, "pool runs"},
		{"lead kernel", []string{"-kernel", "fdn", "-lead", "kernel", "-search-seats", "0"}, "-lead gorge"},
		{"repro", []string{"-kernel", "fdn", "-repro", "x.json", "-search-seats", "0"}, "not -repro"},
		{"v2", []string{"-kernel", "fdn", "-v2", "transcript", "-search-seats", "0"}, "not -repro"},
		{"seat out of range", []string{"-kernel", "fdn", "-search-seats", "2"}, "seat numbers"},
		{"not a seat", []string{"-kernel", "fdn", "-search-seats", "a"}, "seat numbers"},
		{"unknown search", []string{"-kernel", "fdn", "-search-seats", "0", "-search", "mcts"}, "paranoid or maxn"},
		{"no sims", []string{"-kernel", "fdn", "-search-seats", "0", "-search-sims", "0"}, "-search-sims"},
		{"search without seats", []string{"-kernel", "fdn", "-search", "maxn"}, "need -search-seats"},
	} {
		code, _, stderr := runKFuzz(t, tc.args...)
		if code != 2 || !strings.Contains(stderr, tc.want) || strings.Count(strings.TrimSpace(stderr), "\n") != 0 {
			t.Errorf("%s: exit %d, stderr %q; want exit 2 and a one-line reason with %q", tc.name, code, stderr, tc.want)
		}
	}
}

// TestSearchSeatsPoolRun: an FDN pool pair with seat 0 searching prints the
// header and the per-seat searched count, plays the same twice, and a run
// without -search-seats prints neither.
func TestSearchSeatsPoolRun(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	fdnDecks := oracle.PoolDecks("fdn")
	if _, err := os.Stat(filepath.Join(filepath.Dir(fdnDecks), "pool.tsv")); err != nil {
		t.Skipf("FDN pool absent: %v", err)
	}
	slice := []string{"-kernel", "fdn", "-first-pair", "8", "-pairs", "1", "-seed", "1", "-shrink", "0", "-allow-unverified", "-surface", "autopay", "-policy", "heuristic"}
	with := func(extra ...string) []string { return append(append([]string{}, slice...), extra...) }
	code, plain, stderr := runKFuzz(t, slice...)
	if code != 0 || strings.Contains(plain, "search seats") || strings.Contains(plain, "searched=") {
		t.Fatalf("plain run: exit %d\n%s\n%s", code, plain, stderr)
	}
	code, first, stderr := runKFuzz(t, with("-search-seats", "0", "-search-sims", "8")...)
	if code != 0 || !strings.Contains(first, "search seats: 0 paranoid sims=8") {
		t.Fatalf("search run: exit %d\n%s\n%s", code, first, stderr)
	}
	m := regexp.MustCompile(`searched=\[(\d+) (\d+) 0 0\]`).FindStringSubmatch(first)
	if m == nil || m[1] == "0" || m[2] != "0" {
		t.Fatalf("seat 0 should have searched and seat 1 not:\n%s", first)
	}
	if _, second, _ := runKFuzz(t, with("-search-seats", "0", "-search-sims", "8")...); second != first {
		t.Errorf("a second run differs:\n%s\nvs\n%s", second, first)
	}
}
