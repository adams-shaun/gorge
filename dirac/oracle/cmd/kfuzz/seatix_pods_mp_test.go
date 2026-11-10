//go:build dirac_mp

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFuzzPodsSeatThreeCasts pins the 4-seat cast path for seats past 1: the
// kernel's pool, restricted-mana and cost reads indexed seats by p&1, so seat
// 3's pool was read as seat 1's, a cast it had tapped for was reversed and the
// card stayed in hand (projection Cards[*].Zone). Pod 80 (heuristic) is seat
// 3's Llanowar Elves off a Forest at decision 10; pod 1 (random) is seat 3's
// Teach by Example at decision 191. Both now play past that point in lockstep.
func TestFuzzPodsSeatThreeCasts(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-fdn")
	for _, c := range []struct {
		pod, policy, card string
	}{
		{"80", "heuristic", "Cards[139].Zone"},
		{"1", "random", "Cards[157].Zone"},
	} {
		code, stdout, stderr := runKFuzz(t, "-synth", "std-fdn", "-seats", "4", "-pods", "1", "-first-pair", c.pod,
			"-policy", c.policy, "-seed", "1", "-shrink", "0")
		if code > 1 {
			t.Fatalf("pod %s: exit %d, stderr %q", c.pod, code, stderr)
		}
		if !strings.Contains(stdout, "std-fdn pods (4 seats): 1 games") {
			t.Fatalf("pod %s: no pod census line:\n%s", c.pod, stdout)
		}
		if strings.Contains(stdout, "projection "+c.card) {
			t.Errorf("pod %s (%s): seat 3's cast still diverges on %s:\n%s", c.pod, c.policy, c.card, stdout)
		}
	}
}
