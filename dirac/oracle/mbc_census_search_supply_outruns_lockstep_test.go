package oracle

import (
	"testing"
	"time"
)

// CR 602.2b: a land taps once for one mana, so a ten-mana ability (The World
// Tree's) beside a few lands is unpayable by any prefix of mana activations,
// yet the relaxed census proof counts a land's granted {T} ability beside its
// printed one and passed it. gorge then ran its prefix search (160 clones a
// walk, three walks per priority decision) to the node budget every time
// (tail-08 pair 3: 59s of gorge CPU, past the lockstep run's 30s game watchdog
// twice, recorded as a hang). A census whose every mana ability is a lone {T}
// for one mana now proves the ability outruns the pool plus one mana per
// source and skips the search; the search slot is still counted, because
// dirac mirrors the slot accounting. The recording replays clean either way
// (the answers were never wrong), so the bound is the pin: past it the search
// runs for plays the supply count refuses again.
func TestMBCTail08CensusSearchLockstep(t *testing.T) {
	t.Parallel()
	start := time.Now()
	stdReproNamed(t, "census_search_supply_outruns")
	if d := time.Since(start); d > 45*time.Second {
		t.Fatalf("the census-search game replayed in %s: the prefix search runs for an ability the supply count proves unpayable again", d)
	}
}
