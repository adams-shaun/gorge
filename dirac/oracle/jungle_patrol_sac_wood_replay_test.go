package oracle

import "testing"

// Jungle Patrol's "Sacrifice a token named Wood: Add {R}" used to price
// differently on the two sides: gorge's potential bound counted the
// production once and the kernel's census priced it as a gap. gorge's bound
// now repeats it once per extra candidate (engine/rules/potential_repeat_fee.go
// addPotentialSacRepeat, CR 605.1a) and the kernel's plain potential fold
// repeats the fixed production the same way (dirac/legal.go sacAnyRepeatUnits,
// CR 107.4a; without it the echo_pay_offer recording in TestReproManifest
// diverges). The recorded buyback game (Invulnerability, tail-14-c000 pair 8
// decision 614) replays clean.
func TestJunglePatrolSacWoodReplaysClean(t *testing.T) {
	res := replayRawRepro(t, "jungle_patrol_sac_wood_mana_counted_once_unmodelled")
	if res.Div != nil {
		t.Fatalf("the recorded game diverges: %+v", res.Div)
	}
	if res.StallUnmodelled {
		t.Fatal("the recorded game stalls unmodelled")
	}
}
