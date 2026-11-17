package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// TestReproReplaysACustomPoolRecording: a recording whose pool is "custom" (the
// table its own decks need, no registered pool holds it) replays from the CLI
// without a -kernel value, and `-kernel custom` is accepted for it too. The
// replay used to refuse it with "replay it with -kernel custom", a value the
// flag then rejected.
func TestReproReplaysACustomPoolRecording(t *testing.T) {
	if _, err := cardset.CorpusDir(); err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	path := filepath.Join("..", "..", "testdata", "repro", "convoke_pip_pick_x_ask_ceiling.repro.json")
	rep, err := oracle.ReadRepro(path)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Kernel != "custom" || rep.Pool != "custom" {
		t.Fatalf("precondition: recording is kernel %q pool %q, want custom", rep.Kernel, rep.Pool)
	}
	for _, args := range [][]string{{"-repro", path}, {"-kernel", "custom", "-repro", path}} {
		code, stdout, stderr := runKFuzz(t, args...)
		if code != 0 || !strings.Contains(stdout, "repro: no divergence") {
			t.Errorf("kfuzz %v: exit %d, want 0 and a clean replay (stdout %q stderr %q)", args, code, stdout, stderr)
		}
	}
	// -kernel custom is only a replay: a sweep has no table to deal from.
	if code, _, stderr := runKFuzz(t, "-kernel", "custom"); code != 2 || !strings.Contains(stderr, "-repro") {
		t.Errorf("-kernel custom without -repro: exit %d, stderr %q; want 2 naming -repro", code, stderr)
	}
}
