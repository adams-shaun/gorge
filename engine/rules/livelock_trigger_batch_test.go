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

// A declared attack with a hundred attackers under trigger multipliers
// (Isshin, Two Heavens as One and kin) queues one TriggerPush per
// (attacker, copy) of the same source ability: hundreds of back-to-back
// pushes that differ only in the stack object each mints (seed 1901 of the
// hunt: 107 attackers, 400+ pushes of one Winota-style trigger in a row). A
// TriggerPush mints a fresh face-less ability object its own payload does
// not name, so a back-to-back run of them is a finite batch, not a period-1
// cycle; a loop that interleaves pushes with other events still repeats.

func feedTriggerPushes(t *testing.T, w *livelockWatcher, seq *uint64, n int, between *events.Event) {
	t.Helper()
	for i := 0; i < n; i++ {
		*seq++
		w.observeFrom(&events.Event{Seq: *seq, Kind: events.TriggerPush, Player: 2, Obj: 258, Text: "triggered ability"}, 0, 0)
		if between != nil {
			*seq++
			ev := *between
			ev.Seq = *seq
			w.observeFrom(&ev, 0, 0)
		}
	}
}

// TestLivelockWatcherTriggerPushBatchWithClockTicksIsNotACycle: 3x the default cycle
// threshold of identical back-to-back TriggerPush events (and the same with
// a ClockTick, which the watcher elides, between each) must not trip.
func TestLivelockWatcherTriggerPushBatchWithClockTicksIsNotACycle(t *testing.T) {
	const n = 3 * defaultCycleEvents
	for name, between := range map[string]*events.Event{
		"back to back":     nil,
		"clock tick split": {Kind: events.ClockTick},
	} {
		t.Run(name, func(t *testing.T) {
			w := newLivelockWatcher(nil)
			var seq uint64
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%d identical trigger pushes tripped the watcher at seq %d: %v", n, seq, r)
				}
			}()
			feedTriggerPushes(t, &w, &seq, n, between)
		})
	}
}

// TestLivelockWatcherTriggerPushInterleavedLoopStillTrips: the exemption is for a run
// of pushes only. A push that repeats with another event between each (a
// trigger that re-triggers itself and resolves) is still a period-2 cycle.
func TestLivelockWatcherTriggerPushInterleavedLoopStillTrips(t *testing.T) {
	w := newLivelockWatcher(nil)
	var seq uint64
	defer func() {
		r := recover()
		le, ok := r.(*LivelockError)
		if !ok || le.Reason != "repeating cycle" {
			t.Fatalf("push/resolve loop did not trip as a repeating cycle (recovered %v)", r)
		}
	}()
	feedTriggerPushes(t, &w, &seq, 3*defaultCycleEvents, &events.Event{Kind: events.Resolve, Obj: state.ObjID(258)})
}
