package oracle

import "testing"

// CR 602.2b: a land taps once for one mana, so a ten-mana ability (The World
// Tree's) beside a few lands is unpayable by any prefix of mana activations,
// yet the relaxed census proof counts a land's granted {T} ability beside its
// printed one and passed it. gorge then ran its prefix search (160 clones a
// walk, three walks per priority decision) to the node budget every time
// (tail-08 pair 3: 59s of gorge CPU, past the lockstep run's 30s game watchdog
// twice, recorded as a hang). A census whose every mana ability is a lone {T}
// for one mana now proves the ability outruns the pool plus one mana per
// source and skips the search; the search slot is still counted, because
// dirac mirrors the slot accounting.
//
// The recording replays clean with or without the skip (the answers were never
// wrong), so this test pins only that skipping the search hides no play dirac
// offers. That the search is skipped is pinned by work, not by wall-clock, in
// engine/rules: TestPotentialCensusFallbackSkipsASearchTheSupplyCountRefuses
// (the search count) and the census.outruns tests in
// potential_plan_supply_skip_test.go.
func TestMBCTail08CensusSearchLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "census_search_supply_outruns")
}
