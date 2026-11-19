package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestLivelockWatcherDelayedRemoveBatchIsNotACycle pins the hunt's cmdr4 seed
// 118786 shape: a few hundred copies of Jace, Cunning Castaway each register
// an end-of-turn delayed trigger, and the next turn change expires every one
// in a single object-less run of delayed_remove events that differ only in the
// registration id they consume. That finite batch is not a loop; the same id
// removed over and over still is.
func TestLivelockWatcherDelayedRemoveBatchIsNotACycle(t *testing.T) {
	const n = 500 // past the default 400-event cycle threshold
	w := newLivelockWatcher(nil)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("%d removals of %d distinct registrations tripped the watcher: %v", n, n, r)
			}
		}()
		for i := 0; i < n; i++ {
			w.observeFrom(&events.Event{Seq: uint64(i + 1), Kind: events.DelayedRemove, Amount: int32(100 + i)}, 0, 0)
		}
	}()

	w = newLivelockWatcher(nil)
	defer func() {
		r := recover()
		if _, ok := r.(*LivelockError); !ok {
			t.Fatalf("%d removals of ONE registration did not trip the watcher (recovered %v)", n, r)
		}
	}()
	for i := 0; i < n; i++ {
		w.observeFrom(&events.Event{Seq: uint64(i + 1), Kind: events.DelayedRemove, Amount: 100}, 0, 0)
	}
}
