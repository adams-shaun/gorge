package oracle

import (
	"sync/atomic"
	"testing"
	"time"
)

// The tests use a 1 s watchdog: a stalled attempt costs the full window, and a
// non-stalled attempt (a bare return) finishes far inside it even on a loaded
// box, so no PASS/FAIL hangs on a short wall-clock window.

// stallingPlay counts its calls and blocks (until the test ends) on the calls
// stall names; the others return 7.
func stallingPlay(t *testing.T, calls *atomic.Int32, stall func(n int32) bool) func() int {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	return func() int {
		if stall(calls.Add(1)) {
			<-release
			return -2
		}
		return 7
	}
}

// A first attempt that stalls past the watchdog is played once more: a host stall
// is not a hang.
func TestWithinTimeoutRetryRecoversFromStall(t *testing.T) {
	var calls atomic.Int32
	play := stallingPlay(t, &calls, func(n int32) bool { return n == 1 })
	got := WithinTimeoutRetry(time.Second, play, -1)
	if n := calls.Load(); n != 2 {
		t.Fatalf("play ran %d times, want 2 (the first attempt timed out)", n)
	}
	if got != 7 {
		t.Errorf("result %d, want 7 from the retry", got)
	}
}

// A game that stalls on both attempts is a hang, and is not tried a third time.
func TestWithinTimeoutRetryReportsDoubleStall(t *testing.T) {
	var calls atomic.Int32
	play := stallingPlay(t, &calls, func(int32) bool { return true })
	got := WithinTimeoutRetry(time.Second, play, -1)
	if got != -1 {
		t.Errorf("result %d, want hung (-1)", got)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("play ran %d times, want 2", n)
	}
}

// A zero timeout waits for the one attempt.
func TestWithinTimeoutRetryZeroTimeoutWaits(t *testing.T) {
	var calls atomic.Int32
	play := stallingPlay(t, &calls, func(int32) bool { return false })
	if got := WithinTimeoutRetry(0, play, -1); got != 7 {
		t.Errorf("result %d, want 7", got)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("play ran %d times, want 1", n)
	}
}
