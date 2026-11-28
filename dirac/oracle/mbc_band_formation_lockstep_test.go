package oracle

import "testing"

const batteringRamBandRepro = "battering_ram_band_formation_unmodelled"

// Battering Ram grants itself banding at the beginning of combat. gorge forms
// the band an accepted CR 702.22c declaration names (engine/rules/banding.go);
// the kernel forms it too now (dirac/combat_band.go: band record, the
// 702.22h block expansion and the 702.22j-k damage choosers), so the recorded
// game (tail-00-c000, autopay, seed 239092, pair 0) replays clean and the
// card is no longer refused.
func TestMBCTail00ReprosLockstep(t *testing.T) {
	res := replayRawRepro(t, batteringRamBandRepro)
	if !res.Clean() {
		t.Fatalf("Battering Ram's band game diverged (%d decisions): %v, stall %q unmodelled %v", res.Decisions, res.Div, res.Stall, res.StallUnmodelled)
	}
}
