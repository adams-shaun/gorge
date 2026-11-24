package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLivelockWatcherTriggerPushBatchIsNotACycle pins the hunt's cmdr4 seed
// 143086 shape: Winota, Joiner of Forces triggers once per attacking non-Human
// creature (CR 508.3a/603.2c), and a few hundred Soldier tokens attacking at
// once log a run of trigger_push events that are byte-identical -- the event
// names the source and the declared attackers, not which attacker this
// instance answers. That finite batch is not a loop; a push/resolve rhythm
// that returns to the same stack depth every period still is.
func TestLivelockWatcherTriggerPushBatchIsNotACycle(t *testing.T) {
	const n = 500 // past the default 400-event cycle threshold
	push := func(i int) events.Event {
		return events.Event{Seq: uint64(i + 1), Kind: events.TriggerPush, Player: 3, Obj: 334,
			IDs: []state.ObjID{349, 301, 425}, Text: "triggered ability"}
	}
	w := newLivelockWatcher(nil)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("%d consecutive trigger pushes tripped the watcher: %v", n, r)
			}
		}()
		for i := 0; i < n; i++ {
			ev := push(i)
			w.observeFrom(&ev, 0, 0)
		}
	}()

	// A real loop interleaves a resolution with each push: the run resets, so
	// every period repeats exactly and the watcher still fires.
	w = newLivelockWatcher(nil)
	defer func() {
		r := recover()
		if _, ok := r.(*LivelockError); !ok {
			t.Fatalf("%d push/resolve pairs did not trip the watcher (recovered %v)", n, r)
		}
	}()
	for i := 0; i < n; i++ {
		ev := push(2 * i)
		w.observeFrom(&ev, 0, 0)
		res := events.Event{Seq: uint64(2*i + 2), Kind: events.Resolve, Player: 3, Obj: 900}
		w.observeFrom(&res, 0, 0)
	}
}
