package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const patchworkCrawlerGainedLimitRepro = "patchwork_crawler_gained_once_each_turn_ability_unmodelled"

// CR 613.1f with CR 602.5b: Patchwork Crawler has the activated abilities of the
// creature cards exiled with it, so Stromkirk Condemned's "activate only once
// each turn" pump becomes the Crawler's, limit and all. Gorge counts no gained
// activation against that limit and the kernel's once-each-turn bits belong to
// a printed ability index, so the walk stalls gainsof (tail-03-c001 pair 54,
// static_gainsof.go gainedModelled). The Crawler is refused by name, as
// Thermopod is, and the recorded game still shows the stall with it implicated.
func TestMBCTail03ReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Patchwork Crawler" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != patchworkCrawlerGainedLimitRepro {
			t.Fatalf("Patchwork Crawler's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, patchworkCrawlerGainedLimitRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Patchwork Crawler has no dirac/support/refused entry: its gained once-each-turn ability stalls gainsof")
}
