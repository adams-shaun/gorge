package oracle

import "testing"

const fortifiedAreaBandRepro = "fortified_area_band_formation"

// Fortified Area gives Walls banding. gorge forms the band an accepted
// CR 702.22c declaration names (engine/rules/banding.go); the kernel forms it
// too now (dirac/combat_band.go), so the recorded game (tail-13, autopay,
// pair 11 of a Fortified Area focus, seed 2) replays clean and the card is no
// longer refused.
func TestMBCTail13FortifiedAreaBandRefused(t *testing.T) {
	res := replayRawRepro(t, fortifiedAreaBandRepro)
	if !res.Clean() {
		t.Fatalf("Fortified Area's band game diverged (%d decisions): %v, stall %q unmodelled %v", res.Decisions, res.Div, res.Stall, res.StallUnmodelled)
	}
}
