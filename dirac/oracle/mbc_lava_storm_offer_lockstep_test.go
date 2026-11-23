package oracle

import "testing"

const lavaStormRepro = "lava_storm_instant_offered_without_combat"

// Lava Storm ({3}{R}{R}) is castable on a board with Cascade Bluffs (a hybrid-fee
// filter land) beside Mage-Ring Network (a storage-counter battery): the Mountain
// pays the Bluffs fee and three Swamps pay the {3} (pool tail-55 pair 46, seed
// 839419, decision 715). Gorge's prefix search (rules/potential_script.go) once
// ran out of its 160-node budget on that board, so the plan was census_incomplete
// and gorge withheld cast(#49{Lava Storm}) while dirac offered it. The mana
// planner changes that arrived with origin/main (potential_script.go,
// potential_plan.go, paid_mana_census.go and the new potential_repeat_fee.go)
// closed the gap: both engines now offer the cast and the recorded game replays
// clean, so Lava Storm is not refused and this test pins that.
func TestMBCLavaStormReprosLockstep(t *testing.T) {
	t.Parallel()
	res := replayRawRepro(t, lavaStormRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", lavaStormRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", lavaStormRepro, res.Stall)
	}
}
