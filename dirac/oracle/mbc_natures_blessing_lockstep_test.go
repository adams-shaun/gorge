package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const naturesBlessingBandRepro = "natures_blessing_band_formation_unmodelled"

// Nature's Blessing gives a creature banding for good (Duration$ Permanent).
// gorge forms the band an accepted CR 702.22c declaration names
// (engine/rules/banding.go); the kernel poses the same ask and mirrors the
// decline, but band formation (CR 702.22d-k: the band blocked as a group, the
// damage divisions) is not modelled, so an accepted candidate stalls it at
// StallBand (dirac/combat_band.go). The recorded game (tail-20-c000, autopay,
// seed 961864, pair 12) stalls there after 561 decisions; it is expected to
// stay unmodelled, so the entry holds the game rather than a passing replay.
// The rest of that deal (Hidden Hideout, Pride of the Clouds) plays clean.
func TestMBCTail20NaturesBlessingBandRefused(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Nature's Blessing" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != naturesBlessingBandRepro {
			t.Fatalf("Nature's Blessing's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, naturesBlessingBandRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Nature's Blessing has no dirac/support/refused entry: an accepted band stalls the kernel (StallBand)")
	}
}
