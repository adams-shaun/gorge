package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// TestFuzzPoolRunWritesEveryBadGameRepro: a pool run with -out writes every
// divergent game's repro, not only the first's, and the file alone replays the
// game (its own cards, no re-deal of the pool).
func TestFuzzPoolRunWritesEveryBadGameRepro(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-fdn")
	mutateKernel(true)
	defer mutateKernel(false)
	dir := t.TempDir()
	code, stdout, stderr := runKFuzz(t, "-synth", "std-fdn", "-play-refused", "-first-pair", "31", "-pairs", "3", "-seed", "1", "-keep-going", "-surface", "autopay", "-shrink", "0", "-implicates-all", "-out", dir)
	if code > 1 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	bad := map[int]bool{}
	var replay string
	for _, line := range strings.Split(stdout, "\n") {
		if !strings.HasPrefix(line, "bad-game\t") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 8 {
			t.Fatalf("bad-game line has %d fields, want 8: %q", len(f), line)
		}
		pair, _ := strconv.Atoi(f[1])
		bad[pair] = true
		if f[3] != "divergent" {
			continue
		}
		seed, err := strconv.ParseUint(f[2], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, fmt.Sprintf("divergent-pair%d.repro.json", pair))
		r, err := oracle.ReadRepro(path)
		if err != nil {
			t.Fatalf("pair %d: %v", pair, err)
		}
		if len(r.Actions) == 0 || r.Seed != seed || r.Kernel != "synth std-fdn" || r.Pool != "std-fdn" {
			t.Errorf("pair %d repro: %d actions, seed %d (want %d), kernel %q, pool %q", pair, len(r.Actions), r.Seed, seed, r.Kernel, r.Pool)
		}
		if replay == "" {
			replay = path
		}
	}
	if len(bad) < 2 || replay == "" {
		t.Fatalf("fixture moved: want at least 2 divergent bad-game lines in pairs 31-33, got %v:\n%s", bad, stdout)
	}
	t.Logf("bad pairs %v", bad)
	first, err := oracle.ReadRepro(filepath.Join(dir, "repro.json"))
	if err != nil {
		t.Fatalf("first-divergence repro: %v", err)
	}
	if first.Kernel != "synth std-fdn" || first.Pool != "std-fdn" {
		t.Errorf("repro.json: kernel %q, pool %q; want synth std-fdn, std-fdn", first.Kernel, first.Pool)
	}
	for p := 31; p <= 33; p++ {
		if bad[p] {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(dir, fmt.Sprintf("*-pair%d.repro.json", p)))
		if len(matches) != 0 {
			t.Errorf("clean pair %d has repros %v", p, matches)
		}
	}
	if code, out, stderr := runKFuzz(t, "-synth", "std-fdn", "-play-refused", "-repro", replay); code != 1 {
		t.Errorf("mutated replay of %s: exit %d, want 1 (diverges)\nstdout %s\nstderr %s", replay, code, out, stderr)
	}
	mutateKernel(false)
	if code, out, stderr := runKFuzz(t, "-synth", "std-fdn", "-play-refused", "-repro", replay); code != 0 {
		t.Errorf("clean replay of %s: exit %d, want 0\nstdout %s\nstderr %s", replay, code, out, stderr)
	}
}
