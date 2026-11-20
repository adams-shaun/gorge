package oracle

import (
	"strings"
	"testing"
)

// Four-seat edh pods (kfuzz -seed 2, full 100-card precons) that stopped at a
// kernel stall gorge plays on. Each recording holds its four decks.

// fourSeatNoStall fails unless the replay of name plays past decision at with no
// divergence and no kernel stall.
func fourSeatNoStall(t *testing.T, name string, at int) {
	t.Helper()
	res := fourSeatEdhReplay(t, name)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stalled: %q", res.Stall)
	}
	if res.Decisions <= at {
		t.Fatalf("%d decisions, the game must go past decision %d", res.Decisions, at)
	}
}

// Run Away Together ("choose two target creatures controlled by different
// players, return those creatures to their owners' hands") on a commander and a
// creature. A commander about to go to its owner's hand may go to the command
// zone instead (CR 903.9b), a replacement matched before the move, so gorge
// parks the commander's bounce and asks its owner before either creature
// moves. The kernel's all-at-once bounce (leaveTogether) moved the commander
// with no park (StallLowering), and its per-target loop moved the other target
// before the park, so the answer came too late. A commander among the
// targets now takes the per-target path, which stops at the park.

// TestFourSeatRunAwayTogetherCommanderBounceLockstep: the bounce asks the
// commander's owner first, with the other target still on the battlefield.
func TestFourSeatRunAwayTogetherCommanderBounceLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	fourSeatNoStall(t, "run_away_together_commander_bounce_four_seat_pod", 765)
}

// TestFourSeatRunAwayTogetherCommanderBounceSecondLockstep: the second pod that
// reached the same bounce.
func TestFourSeatRunAwayTogetherCommanderBounceSecondLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	fourSeatNoStall(t, "run_away_together_commander_bounce_second_four_seat_pod", 956)
}
