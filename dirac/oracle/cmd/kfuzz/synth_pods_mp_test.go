//go:build dirac_mp

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFuzzSynthPods plays a synthetic pool's decks as 4-seat pods against the
// real kernel table: every pod starts a game, the pod census prints with the
// twoseat stalls apart from capacity, its counts add up, and two runs agree.
func TestFuzzSynthPods(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-fdn")
	args := []string{"-synth", "std-fdn", "-seats", "4", "-pods", "6", "-seed", "1", "-keep-going", "-shrink", "0", "-workers", "2"}
	code, stdout, stderr := runKFuzz(t, args...)
	if code > 1 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	re := regexp.MustCompile(`std-fdn pods \(4 seats\): (\d+) games, (\d+) clean, (\d+) divergent, (\d+) capacity, (\d+) twoseat, (\d+) unmodelled, (\d+) hang`)
	g := re.FindStringSubmatch(stdout)
	if g == nil {
		t.Fatalf("no pod census line:\n%s", stdout)
	}
	n := make([]int, len(g)-1)
	for i := range n {
		n[i], _ = strconv.Atoi(g[i+1])
	}
	if n[0] != 6 {
		t.Errorf("%d games started, want 6:\n%s", n[0], stdout)
	}
	if sum := n[1] + n[2] + n[3] + n[4] + n[5] + n[6]; sum != n[0] {
		t.Errorf("census sums to %d of %d games: %s", sum, n[0], g[0])
	}
	if !strings.Contains(stdout, "std-fdn: 6 games (") {
		t.Errorf("verdict line missing:\n%s", stdout)
	}
	_, again, _ := runKFuzz(t, args...)
	if again != stdout {
		t.Errorf("two runs differ:\n%s\n---\n%s", stdout, again)
	}
}

// TestFuzzPodsGate: a pod against a kernel other than vanilla or a -synth pool
// is refused, and the message names what does play.
func TestFuzzPodsGate(t *testing.T) {
	code, _, stderr := runKFuzz(t, "-kernel", "fdn", "-seats", "4", "-pods", "1")
	if code != 2 || !strings.Contains(stderr, "-synth pool") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}
