package oracle

import (
	"sync/atomic"
	"testing"
	"time"
)

// A first attempt that stalls past the watchdog is played once more, and the
// retry's result is the game's: a host stall is not a property of the game.
func TestWithinTimeoutRetryRecovers(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	got := WithinTimeoutRetry(20*time.Millisecond, func() int {
		if calls.Add(1) == 1 {
			<-release
		}
		return 7
	}, -1)
	if got != 7 {
		t.Fatalf("result %d, want the retry's 7", got)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("played %d times, want 2 (one retry, no more)", n)
	}
}

// A game that stalls on both attempts is a hang, and the watchdog stops after
// the one retry rather than playing it again.
func TestWithinTimeoutRetryGivesUpAfterTwo(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	got := WithinTimeoutRetry(20*time.Millisecond, func() int {
		calls.Add(1)
		<-release
		return 7
	}, -1)
	if got != -1 {
		t.Fatalf("result %d, want hung -1", got)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("played %d times, want 2 (one retry, no more)", n)
	}
}

// A zero timeout is no watchdog: the first attempt is waited for and never
// retried.
func TestWithinTimeoutRetryNoWatchdogDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	got := WithinTimeoutRetry(0, func() int {
		calls.Add(1)
		return 7
	}, -1)
	if got != 7 {
		t.Fatalf("result %d, want 7", got)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("played %d times, want 1", n)
	}
}
