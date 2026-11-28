package oracle

import "testing"

const naturesBlessingBandRepro = "natures_blessing_band_formation_unmodelled"

// Nature's Blessing gives a creature banding for good (Duration$ Permanent).
// gorge forms the band an accepted CR 702.22c declaration names
// (engine/rules/banding.go); the kernel forms it too now
// (dirac/combat_band.go), so the recorded game (tail-20-c000, autopay, seed
// 961864, pair 12) replays clean and the card is no longer refused. The rest
// of that deal (Hidden Hideout, Pride of the Clouds) plays clean.
func TestMBCTail20NaturesBlessingBandRefused(t *testing.T) {
	res := replayRawRepro(t, naturesBlessingBandRepro)
	if !res.Clean() {
		t.Fatalf("Nature's Blessing's band game diverged (%d decisions): %v, stall %q unmodelled %v", res.Decisions, res.Div, res.Stall, res.StallUnmodelled)
	}
}
