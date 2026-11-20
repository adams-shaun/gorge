package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const eviscerator528Repro = "flashback_graveyard_cast_phased_prefix_unmodelled"

// gorge's prefix search falls back to a phased run (potential_script.go
// scriptSearch.phased) for a cast whose own cost sacrifices a permanent:
// Eviscerator's Insight's {4}{B} flashback through Elven Chorus's grant floats
// the granted taps first, sacrifices the four Eldrazi Spawn, taps the Forest
// and pays Prismite's {2} filter twice (a 17-step prefix). The kernel's
// grantSrch has only the plain run, whose planner taps another source set, so
// the recorded game (tail-51 pair 34) parts one decision after the offer at
// decision 522 (projection Cards[6].Flags at 528). Porting the phased run is
// over 200 lines of dirac, so the card is refused by name and the recording
// still shows the divergence with it implicated.
func TestMBCEvisceratorSInsightPhasedPrefixRefused(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Eviscerator's Insight" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != eviscerator528Repro {
			t.Fatalf("Eviscerator's Insight's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, eviscerator528Repro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Eviscerator's Insight has no dirac/support/refused entry: its flashback payment parts from gorge's phased prefix search")
}
