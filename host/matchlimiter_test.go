package host

import (
	"sync"
	"testing"
	"time"
)

// takeWithin runs l.take(t) n times on another goroutine and reports whether
// all of them returned within d. On a timeout it drains the limiter so the
// stuck goroutine can finish, then leaves it permanently refilled so the
// failing test's own slot releases cannot hang in their turn: a broken
// limiter fails in d, never at the 10-minute test timeout.
func takeWithin(t *testing.T, l *matchLimiter, n int, d time.Duration) bool {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < n; i++ {
			l.take(t)
		}
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		for {
			select {
			case <-l.slots:
			case <-done:
				go func() {
					for {
						l.slots <- struct{}{}
					}
				}()
				return false
			}
		}
	}
}

// TestMatchLimiterNeverHoldsAndWaits is the host-package hang
// (TestUndoRefusedOnTwoHumanTables holding one of the 8 slots and waiting
// for a second while every other slot was held by a test parked on a human
// decision) reduced to the limiter: every slot but one is held by other
// tests, and one test builds two registries. It must not wait for a second
// slot while holding the first.
func TestMatchLimiterNeverHoldsAndWaits(t *testing.T) {
	const size = 8
	l := newMatchLimiter(size)
	for i := 0; i < size-1; i++ {
		l.slots <- struct{}{} // the other parallel tests, parked on human decisions
	}
	if !takeWithin(t, l, 2, 5*time.Second) {
		t.Fatal("a test's second testOptions waited for a slot while holding one (hold-and-wait deadlock)")
	}
	if got := len(l.slots); got != size {
		t.Fatalf("one test holds %d slots, want exactly 1", got-(size-1))
	}
	for i := 0; i < size-1; i++ {
		<-l.slots
	}
}

// TestMatchLimiterSequentialRegistriesFitOneSlot: eight sequential
// testOptions calls on one t (TestHostedPoliciesReplayDeterministically's
// shape) must fit a limiter smaller than eight -- the -race size is 4.
func TestMatchLimiterSequentialRegistriesFitOneSlot(t *testing.T) {
	l := newMatchLimiter(4)
	if !takeWithin(t, l, 8, 5*time.Second) {
		t.Fatal("eight testOptions calls on one test exhausted a 4-slot limiter")
	}
}

// TestMatchLimiterSubtestsShareTheirRootsSlot: parallel subtests of one
// top-level test share its slot, and the slot is returned only after the
// last of them finishes.
func TestMatchLimiterSubtestsShareTheirRootsSlot(t *testing.T) {
	l := newMatchLimiter(1)
	var both sync.WaitGroup
	both.Add(2)
	t.Run("group", func(t *testing.T) {
		for _, name := range []string{"a", "b"} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				if !takeWithin(t, l, 1, 5*time.Second) {
					t.Error("a subtest waited for its sibling's slot")
				}
				both.Done()
				ch := make(chan struct{})
				go func() { both.Wait(); close(ch) }()
				select {
				case <-ch:
				case <-time.After(5 * time.Second):
					t.Error("the subtests did not hold the shared slot concurrently")
				}
			})
		}
	})
	if got := len(l.slots); got != 0 {
		t.Fatalf("%d slots still held after every subtest finished", got)
	}
}
