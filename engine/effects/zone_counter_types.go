package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// emitChangeZoneCounters applies every counter kind in Forge's comma-delimited
// WithCountersType$ list. The order is the script order, keeping replay and
// replacement processing deterministic. Each kind is folded through
// state.CanonicalCounterKind first: a script string is not a counter kind
// until it is normalised, so `WithCounters$ Shield` places the real SHIELD
// counter rather than the engine's mixed-case regeneration marker.
func emitChangeZoneCounters(h Host, id state.ObjID, kinds string, amount int32) {
	for _, kind := range strings.Split(kinds, ",") {
		if kind = state.CanonicalCounterKind(strings.TrimSpace(kind)); kind != "" {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: amount})
		}
	}
}

// entryWithCounters folds a battlefield entry's WithCountersType$ list into
// the entry event itself (events.EntryCounterPairs), so the permanent ENTERS
// with the counters (CR 122.6, 614.1c) instead of getting them after it
// arrived: an "enters with power 4 or greater" trigger sees the counters, and
// the rules emit path still settles them through the AddCounter replacements
// with the entry's other grants (engine/rules/entry_counters.go). A MoveZone
// entry and a token mint (TokenCreate/CardToken) share the shape: both fold
// their Pairs in events.Apply, and both are read by the entry-counter plan.
// It reports whether it took over the placement; an exile move (suspend's TIME
// counters) keeps the post-move emitChangeZoneCounters.
func entryWithCounters(ev *events.Event, to state.Zone, kinds string, amount int32) bool {
	if kinds == "" {
		return false
	}
	switch ev.Kind {
	case events.MoveZone:
		if to != state.ZBattlefield {
			return false
		}
	case events.TokenCreate, events.CardToken:
	default:
		return false
	}
	for _, kind := range strings.Split(kinds, ",") {
		if kind = state.CanonicalCounterKind(strings.TrimSpace(kind)); kind != "" {
			ev.Pairs = append(ev.Pairs, events.EntryCounterPairs(events.EntryCounterGrant{Kind: kind, Amount: amount})...)
		}
	}
	return true
}
