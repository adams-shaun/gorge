package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFuzzFullFiltDeadEndSettlesPassSteps replays the std-tmt pair where
// gorge casts Go Ninja Go ({R}{W}) at p1's priority with two Mountains in
// play and no white source: the {W} comes from Omni-Cheese Pizza's
// "{1}, {T}, sacrifice: add one mana of any colour", a first-ability filter.
// The sacrifice triggers Super Shredder's leaves-the-battlefield counter
// (another permanent left), so the filter search's path ends in a pass step
// before the sorcery-speed cast. The dead-end replay used to leave that
// trigger stacked on its fork, where the sorcery gate drops the cast, and the
// cast was never offered.
func TestFuzzFullFiltDeadEndSettlesPassSteps(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-tmt")
	code, stdout, stderr := runKFuzz(t, "-synth", "std-tmt", "-first-pair", "164", "-pairs", "1", "-seed", "1", "-surface", "full", "-shrink", "0", "-mulligans", "1", "-absolute")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q\n%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "1 clean, 0 divergent") {
		t.Errorf("pair 164 diverged:\n%s", stdout)
	}
}
