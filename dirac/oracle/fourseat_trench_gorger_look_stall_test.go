package oracle

import (
	"strings"
	"testing"
)

// trenchGorgerRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed 3,
// pod 286): Trench Gorger's "search for any number of lands" takes 25 picks, one
// past the kernel's 24-card look window (genState.look). gorge takes them all.
const trenchGorgerRepro = "trench_gorger_search_past_look_window_four_seat_pod"

// TestFourSeatTrenchGorgerSearchPastLookWindow replays the pod: the 25th pick is
// a declared capacity stall (StallLook, counted), not an index-out-of-range
// panic that the lockstep reports as a divergence. The window cannot grow
// (State +0 B), so the stall is the declared limit.
func TestFourSeatTrenchGorgerSearchPastLookWindow(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, trenchGorgerRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if !strings.HasPrefix(res.Stall, KernelStallPrefix) || !strings.Contains(res.Stall, "look") {
		t.Fatalf("stall %q, want the declared look-window stall", res.Stall)
	}
}
