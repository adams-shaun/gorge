//go:build spellbench

package sbv2http

import (
	"context"
	"runtime"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/humanseat"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
)

// A driver the codec has cancelled can still reach the hub after the driver that
// replaced it. It must not touch the live request: closing that makes the live driver
// give the decision up, and the stale request closes on its own cancelled context, so
// a v2 client is left with nothing to answer and the game waits on it for good.
func TestAStaleDriverLeavesTheLiveRequestOpen(t *testing.T) {
	ss := newSeatState()
	ask := humanseat.Ask{Pose: &mapping.Pose{Candidates: make([]mapping.Cand, 1)}}

	type chosen struct {
		i   int
		err error
	}
	live := make(chan chosen, 1)
	go func() {
		i, err := ss.Choose(context.Background(), ask)
		live <- chosen{i, err}
	}()
	var open *request
	for open == nil {
		open, _ = ss.snapshot()
		runtime.Gosched()
	}

	stale, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ss.Choose(stale, ask); err != context.Canceled {
		t.Fatalf("a stale Choose returned %v", err)
	}
	ss.NativeOnly(stale, humanseat.NativeOnly{})

	if now, _ := ss.snapshot(); now != open {
		t.Fatalf("a stale driver replaced the live request: %+v", now)
	}
	if !ss.answer(open.id, 0) {
		t.Fatal("the live request no longer takes an answer")
	}
	if got := <-live; got.err != nil || got.i != 0 {
		t.Fatalf("the live driver got %+v", got)
	}
}
