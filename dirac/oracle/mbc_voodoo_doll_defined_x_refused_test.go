package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const voodooDollDefinedXRepro = "destroyed_source_reads_last_known_counters"

// Voodoo Doll's "{X}{X}, {T}" has its X defined by its pin counters
// (SVar:X:Count$CardCounters.PIN, CR 107.3c): gorge folds the two symbols into
// that much generic mana and announces nothing, while the kernel still carries
// the {X}{X} and asks for it. dirac refuses the card by name rather than lower
// the fixed form in this change (a new own-cost raise that re-evidences the
// card's IR), and the recorded tail-05 game is the game that asks X where
// gorge asks nothing.
func TestMBCTail05VoodooDollDefinedXRefused(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Voodoo Doll" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != voodooDollDefinedXRepro {
			t.Fatalf("Voodoo Doll's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, voodooDollDefinedXRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Voodoo Doll has no dirac/support/refused entry: gorge fixes its text-defined {X}{X} and announces nothing while the kernel asks for it")
	}
}
