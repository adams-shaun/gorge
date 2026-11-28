package oracle

import "testing"

// TestBandStallGroveGapRecordingClean replays grove_gap_flashback_script_budget
// (Ray of Revelation's recording) in lockstep over a raw table of its own
// cards and requires the whole game clean: the recorded prefix served with no
// skip, no divergence, no stall. It is the recording that used to end at
// dirac.StallBand, the unmodelled CR 702.22 band formation; the kernel models
// it now (dirac/combat_band.go).
func TestBandStallGroveGapRecordingClean(t *testing.T) {
	oneOffReproClean(t, "grove_gap_flashback_script_budget")
}
