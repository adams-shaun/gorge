package oracle

import "testing"

// Deepglow Skate's every-kind MultiplyCounter doubles Midnight Clock from 10
// to 20 HOUR counters. A multi-counter put that goes from fewer than twelve
// to more than twelve crosses the twelfth-counter line once (CR 122.6,
// 603.2c; Midnight Clock's 2019-10-04 ruling). The kernel fired one
// CounterAddedOnce with the summed amount and a stale kind, so the kind
// filter dropped the line (four-seat edh pod 35, decision 1121).
const midnightClockDoubledRepro = "edh_midnight_clock_doubled_crosses"

// midnightClockDoubledFirstAt is the decision the recording diverged at
// before the fix.
const midnightClockDoubledFirstAt = 1121

// TestEdhMidnightClockDoubledCrossesLockstep replays the four-seat edh pod in
// lockstep past the recorded decision.
func TestEdhMidnightClockDoubledCrossesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, midnightClockDoubledRepro)
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= midnightClockDoubledFirstAt {
		t.Errorf("%d decisions, the game must go past decision %d", res.Decisions, midnightClockDoubledFirstAt)
	}
}
