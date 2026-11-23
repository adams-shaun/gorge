package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const batteringRamBandRepro = "battering_ram_band_formation_unmodelled"

// Battering Ram grants itself banding at the beginning of combat. gorge forms
// the band an accepted CR 702.22c declaration names (engine/rules/banding.go);
// the kernel poses the same ask and mirrors the decline, but band formation
// (CR 702.22d-k: the band blocked as a group, the damage divisions) is not
// modelled, so an accepted candidate stalls it at StallBand (dirac/combat_band.go).
// The recorded game (tail-00-c000, autopay, seed 239092, pair 0) stalls there
// at decision 309; it is expected to stay unmodelled, so the entry holds the
// game rather than a passing replay.
func TestMBCTail00ReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Battering Ram" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != batteringRamBandRepro {
			t.Fatalf("Battering Ram's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, batteringRamBandRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Battering Ram has no dirac/support/refused entry: an accepted band stalls the kernel (StallBand)")
	}
}
