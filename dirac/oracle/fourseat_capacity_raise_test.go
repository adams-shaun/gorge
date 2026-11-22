package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// The 2026-11-22 capacity raises (maxXTgt 31 -> 63, maxSacOrd 16 -> 64,
// MaxPending and MaxStack 48 -> 96) each answer a four-seat edh pod of the
// seed-2/3 128-pod sweeps that the kernel stopped at a declared capacity stall
// (xtgt, sac, pending) while gorge played on. Each recording was made at that
// stall, so it ends where the stall was; the replay runs 200 decisions past it.
// A test fails if the kernel stalls, if the two sides diverge, or if the game
// does not go past the recorded script.

// capacityPod replays a recording and checks the kernel plays it clean past
// its end, returning the peak occupancy of the pools the raise concerns.
func capacityPod(t *testing.T, name string) (peakXT, peakPend, peakStack int) {
	t.Helper()
	prev := OccupancyHook
	OccupancyHook = func(s *dirac.State) {
		var o dirac.Occupancy
		s.Occupancy(&o)
		peakXT = max(peakXT, o.N[dirac.PoolXTgt])
		peakPend = max(peakPend, o.N[dirac.PoolPending])
		peakStack = max(peakStack, o.N[dirac.PoolStack])
	}
	defer func() { OccupancyHook = prev }()
	res := fourSeatEdhReplay(t, name)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stalled: %q", res.Stall)
	}
	return
}

// TestFourSeatXTgtPast32Lockstep: a spell with "any number of target
// permanents" (Deepglow Skate) picks 40 and 53 objects, past the old 32.
func TestFourSeatXTgtPast32Lockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range []struct {
		name string
		min  int
	}{
		{"capacity_xtgt_past_32_targets_pod55", 31},
		{"capacity_xtgt_past_32_targets_pod60", 31},
	} {
		xt, _, _ := capacityPod(t, c.name)
		if xt <= c.min {
			t.Errorf("%s: peak extra targets %d, want past %d", c.name, xt, c.min)
		}
	}
}

// TestFourSeatSacBatchPast16Lockstep: one effect sacrifices 24 permanents in
// one batch (genState.sacOrd held 16).
func TestFourSeatSacBatchPast16Lockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	capacityPod(t, "capacity_sac_batch_past_16_pod80")
}

// TestFourSeatPendingPast48Lockstep: more than 48 triggers queue at once and
// all go on the stack (CR 603.3b); MaxPending and MaxStack held 48.
func TestFourSeatPendingPast48Lockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	_, pend, stack := capacityPod(t, "capacity_pending_past_48_triggers_pod73")
	if pend <= 48 && stack <= 48 {
		t.Errorf("peak pending %d, stack %d: want one past 48", pend, stack)
	}
}
