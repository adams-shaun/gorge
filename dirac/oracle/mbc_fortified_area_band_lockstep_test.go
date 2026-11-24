package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const fortifiedAreaBandRepro = "fortified_area_band_formation"

// Fortified Area gives Walls banding. gorge forms the band an accepted CR
// 702.22c declaration names (engine/rules/banding.go); the kernel poses the
// same ask and mirrors the decline, but band formation (CR 702.22d-k: the
// band blocked as a group, the damage divisions) is not modelled, so an
// accepted candidate stalls it at StallBand (dirac/combat_band.go). The
// recorded game (tail-13, autopay, pair 11 of a Fortified Area focus, seed 2)
// stalls there after 238 decisions, Valiant Changeling (a Wall by changeling)
// leading a candidate; it is expected to stay unmodelled, so the entry holds
// the game rather than a passing replay.
func TestMBCTail13FortifiedAreaBandRefused(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Fortified Area" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != fortifiedAreaBandRepro {
			t.Fatalf("Fortified Area's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, fortifiedAreaBandRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Fortified Area has no dirac/support/refused entry: an accepted band stalls the kernel (StallBand)")
	}
}
