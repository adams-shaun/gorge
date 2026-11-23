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
	// A pair the table refuses (an as-enters opponent choice beside another
	// object's enters replacement, Spyglass beside Authority of the Consuls) is
	// skipped, so the pods are the played games plus the refused ones.
	vr := regexp.MustCompile(`std-fdn: (\d+) games \((\d+) refused-card, (\d+) refused-pair, (\d+) error\)`).FindStringSubmatch(stdout)
	if vr == nil {
		t.Fatalf("verdict line missing:\n%s", stdout)
	}
	refused := 0
	for _, f := range vr[2:] {
		k, _ := strconv.Atoi(f)
		refused += k
	}
	if n[0] == 0 || n[0]+refused != 6 {
		t.Errorf("%d games played and %d pods refused, want 6 pods in all:\n%s", n[0], refused, stdout)
	}
	if sum := n[1] + n[2] + n[3] + n[4] + n[5] + n[6]; sum != n[0] {
		t.Errorf("census sums to %d of %d games: %s", sum, n[0], g[0])
	}
	if vr[1] != g[1] {
		t.Errorf("verdict line plays %s games, the pods line %s:\n%s", vr[1], g[1], stdout)
	}
	_, again, _ := runKFuzz(t, args...)
	if again != stdout {
		t.Errorf("two runs differ:\n%s\n---\n%s", stdout, again)
	}
}

// TestFuzzPodsGate: a pod against a kernel other than vanilla, a registered
// pool or a -synth pool is refused, and the message names what does play.
func TestFuzzPodsGate(t *testing.T) {
	code, _, stderr := runKFuzz(t, "-kernel", "gorge-api", "-seats", "4", "-pods", "1")
	if code != 2 || !strings.Contains(stderr, "-synth pool") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}
