package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const mindBendRepro = "mind_bend_text_change_reads_word"

// CR 612 with CR 305.6: Mind Bend rewrites a basic land type word on its
// target, so a Mountain turned Island taps for {U}. The kernel holds no
// per-permanent text, so dr-mbc-aaab pair 3 (Mind Bend on a Mountain, Mountain
// to Island) stalled it at textchange. Modelling the rewrite is far past
// bounded dirac code, so Mind Bend is refused by name and its recorded game
// still stalls with it implicated.
func TestMBCDrMbcAaabTextChangeLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Mind Bend" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != mindBendRepro {
			t.Fatalf("Mind Bend's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, mindBendRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Mind Bend has no dirac/support/refused entry: its text change stalls textchange")
	}
}
