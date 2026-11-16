package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const shrineOfBoundlessGrowthRepro = "shrine_of_boundless_growth_potential_offer_unmodelled"

// CR 602.2b: mana abilities may be activated while paying an activation's
// costs. Shrine of Boundless Growth ({T}, Sacrifice: Add {C} for each charge
// counter) funds Resurrection Orb's Equip {4} in gorge, through its potential
// walk: three untapped lands and a two-counter Shrine make the Equip an offer
// the kernel's planner views do not count (decision 579 of tail-34 pair 1).
// The same census counted for casts over-offers them, so the card is refused
// by name, and the recorded game still shows the divergence with it on the
// table. The recording plays on pool one-off, like every refused card's
// recording: a refused card cannot build a table, so no glob may replay it
// (TestStdSetReprosLockstep replays pool custom and fails on any divergence).
func TestMBCResurrectionOrbReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Shrine of Boundless Growth" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != shrineOfBoundlessGrowthRepro {
			t.Fatalf("Shrine of Boundless Growth's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, shrineOfBoundlessGrowthRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Shrine of Boundless Growth has no dirac/support/refused entry: its sacrifice mana funds an offer the kernel does not count")
}
