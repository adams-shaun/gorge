package main

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/oracle"
)

// A game that stalls past the sweep's wall-clock watchdog once is played again
// and its retry's result is kept: a host stall must not be recorded as a hang
// and bar its decks from evidence.
func TestPlayAllRetriesATimedOutGame(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	out, playedN := playAll(1, 1, time.Second, true, func(int) played {
		if calls.Add(1) == 1 {
			<-release
		}
		return played{res: oracle.Result{Over: true}}
	})
	if playedN != 1 {
		t.Errorf("fed %d pairs, want 1 (no hang stopped the feed)", playedN)
	}
	if out[0].hang {
		t.Fatal("a game that finished on its retry was recorded as a hang")
	}
	if !out[0].res.Over {
		t.Fatalf("retry's result not kept: %+v", out[0])
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("played %d times, want 2 (one retry, no more)", n)
	}
}

// Only a game that stalls on both attempts is a hang, and it is not retried a
// second time.
func TestPlayAllHangsOnlyAfterBothAttempts(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	out, playedN := playAll(1, 1, time.Second, true, func(int) played {
		calls.Add(1)
		<-release
		return played{}
	})
	if playedN != 1 {
		t.Errorf("fed %d pairs, want 1", playedN)
	}
	if !out[0].hang {
		t.Fatal("a game that stalled twice was not recorded as a hang")
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("played %d times, want 2 (one retry, no more)", n)
	}
}
