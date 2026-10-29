package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// batchTriggerAlreadyQueued extends the shared action bracket to the two
// aggregate counter modes. The first matching event queues normally; later
// matching recipients accumulate their counter amount and object referent.
func batchTriggerAlreadyQueued(open bool, idxMap *map[triggerKey]int, log *[]millBatchEntry, pendingCount int, t cards.Trigger, key triggerKey, ev *events.Event) bool {
	mode := t.ModeKind()
	if !open || mode != cards.TriggerMilledAll && mode != cards.TriggerTapAll &&
		mode != cards.TriggerUntapAll && mode != cards.TriggerCounterAddedAll &&
		mode != cards.TriggerCounterTypeAddedAll {
		return false
	}
	if *idxMap == nil {
		*idxMap = map[triggerKey]int{}
	}
	if idx, ok := (*idxMap)[key]; ok {
		entry := &(*log)[idx]
		if mode == cards.TriggerCounterAddedAll || mode == cards.TriggerCounterTypeAddedAll {
			entry.amount += ev.Amount
		} else {
			entry.amount++
		}
		if ev.Obj != 0 {
			entry.milled = batchAppendTarget(entry.milled, state.Target{Obj: ev.Obj})
		}
		return true
	}
	amount := int32(1)
	if mode == cards.TriggerCounterAddedAll || mode == cards.TriggerCounterTypeAddedAll {
		amount = ev.Amount
	}
	entry := millBatchEntry{key: key, idx: pendingCount, amount: amount}
	if ev.Obj != 0 {
		entry.milled = batchAppendTarget(entry.milled, state.Target{Obj: ev.Obj})
	}
	(*idxMap)[key] = len(*log)
	*log = append(*log, entry)
	return false
}

// counterBatchKey identifies one CounterPlayerAddedAll cell inside the open
// action batch: the trigger line plus the counter RECIPIENT (an object for
// CounterChange, a player for PlayerCounterChange). Forge's
// GameEntityCounterTable fires the mode once per (putter, entity) cell, so two
// recipients of one PutCounterAll are two triggers, each with its own map.
type counterBatchKey struct {
	triggerKey
	target state.Target
}

// counterBatchEntry records one CounterPlayerAddedAll cell already queued
// inside the open action batch: the pendingTriggers index the single instance
// queued at (the queue is append-only while a batch is open) and the merged
// per-kind counter map every matching event of the action contributed, in
// first-seen event order.
type counterBatchEntry struct {
	key      counterBatchKey
	idx      int
	counters []effects.CounterMapEntry
}

// add merges one matching placement event into the cell's map: the same kind
// accumulates (a CounterTypes$ list may repeat a kind), a new kind appends in
// event order.
func (ent *counterBatchEntry) add(ev *events.Event) {
	for i := range ent.counters {
		if ent.counters[i].Kind == ev.Counter {
			ent.counters[i].Count += ev.Amount
			return
		}
	}
	ent.counters = append(ent.counters, effects.CounterMapEntry{Kind: ev.Counter, Count: ev.Amount})
}

// counterPlayerAddedAllAlreadyQueued is the CounterPlayerAddedAll cell latch
// inside the shared action bracket: the first matching event for a recipient
// returns false (the caller queues the trigger), every later matching event of
// the same action for that recipient merges into the queued entry and returns
// true (the caller skips queueing a second instance). No batch open means the
// event is its own batch-of-one and the fire-time referent already carries the
// one-entry map.
func counterPlayerAddedAllAlreadyQueued(open bool, idxMap *map[counterBatchKey]int, log *[]counterBatchEntry, pendingCount int, t cards.Trigger, key triggerKey, ev *events.Event) bool {
	if !open || t.ModeKind() != cards.TriggerCounterPlayerAddedAll || ev.Counter == "" || ev.Amount <= 0 {
		return false
	}
	var target state.Target
	switch ev.Kind {
	case events.CounterChange:
		if ev.Obj == 0 {
			return false
		}
		target = state.Target{Obj: ev.Obj}
	case events.PlayerCounterChange:
		target = state.Target{Player: ev.Player, IsPlayer: true}
	default:
		return false
	}
	ck := counterBatchKey{triggerKey: key, target: target}
	if *idxMap == nil {
		*idxMap = map[counterBatchKey]int{}
	}
	if idx, ok := (*idxMap)[ck]; ok {
		(*log)[idx].add(ev)
		return true
	}
	ent := counterBatchEntry{key: ck, idx: pendingCount}
	ent.add(ev)
	(*idxMap)[ck] = len(*log)
	*log = append(*log, ent)
	return false
}
