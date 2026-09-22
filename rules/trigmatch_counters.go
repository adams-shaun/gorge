// Counter trigger modes.
//
// Mode$ CounterAdded, CounterAddedOnce, CounterRemoved and CounterRemovedOnce.
//
// Split out of trigger_match.go so tickets touching different modes stop
// colliding on one file. Registration is at the bottom; a duplicate mode
// panics (registerTrigMatcher).

package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// counterAddedMatches implements the "when a counter is put on" trigger
// family for BOTH Mode$ CounterAdded (Shang-Chi and the Ten Rings' "When the
// tenth +1/+1 counter is put on NICKNAME") and Mode$ CounterAddedOnce (Simic
// Ascendancy's "Whenever one or more +1/+1 counters are put on a creature you
// control"). The two modes share this matcher because the engine emits ONE
// CounterChange event per placement batch, with the whole batch in Amount:
// "one trigger per counter-placing event, not per counter" is exactly the
// CounterAddedOnce contract, and the modes differ only in that
// CounterAddedOnce never carries CounterAmount$ (measured: 0 corpus lines)
// while its bodies read TriggerCount$Amount instead. The gate is the
// CounterChange
// event that put counters (Amount > 0: a removal event never adds one).
// CounterType$ names the kind. CounterAmount$ <op><n> is the crossing gate
// the card text means: the trigger fires when the put takes the event's
// counter kind's total on the object from below n to at least n -- the tenth
// counter is put whether one event put 10 or a 5-then-5 pair crossed, and a
// batch that overshoots (9+2) also crossed it. A put that does not cross
// (7+1, 11+1) fires nothing. The threshold ops (EQ/GT/GE) all read as that
// one crossing -- the corpus's CounterAdded CounterAmount$ values are EQ only
// (EQ3..EQ12, every one an oracle "when the <Nth> counter is put" gate), so
// EQ is the measured shape and GT/GE collapse onto it unmeasured. An absent
// CounterAmount$ is Forge's plain "whenever a counter is put" -- every put
// admits it.
func (e *Engine) counterAddedMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if ev.Kind != events.CounterChange || ev.Amount <= 0 {
		return false
	}
	o := e.G.Obj(ev.Obj)
	if o == nil {
		return false
	}
	if kind := t.Params["CounterType"]; kind != "" && !strings.EqualFold(kind, ev.Counter) {
		return false
	}
	if !e.eventCardAndPlayerMatch(t, source, ev.Obj, o.Controller) {
		return false
	}
	if cmp := t.Params["CounterAmount"]; cmp != "" {
		op, n, ok := splitCompare(strings.TrimSpace(cmp))
		if !ok {
			return false
		}
		after := o.Counter(ev.Counter)
		before := after - ev.Amount
		if before < 0 {
			before = 0
		}
		// The crossing semantics for the threshold ops: below n before, at
		// least n after. applyCompare(after, op, n) would miss an overshooting
		// batch (9+2 on EQ10: after=11 is not == 10) and re-fire on every
		// later put that lands exactly on n -- the oracle text ("when the
		// tenth counter is put") fires once, on the crossing, so EQ/GT/GE
		// collapse onto before < n && after >= n. LT/LE/NE do not occur in
		// the corpus on this mode; they keep the plain post-event comparison.
		switch op {
		case "EQ", "GT", "GE":
			if !(before < int32(n) && after >= int32(n)) {
				return false
			}
		default:
			if !applyCompare(int(after), op, n) {
				return false
			}
		}
	}
	return true
}

// counterRemovedMatches is CounterAdded's mirror for Mode$ CounterRemoved AND
// Mode$ CounterRemovedOnce ("whenever a counter is removed from ~", "when the
// last <kind> counter is removed from ~", "whenever one or more counters are
// removed from ~"): the event is a CounterChange with a NEGATIVE Amount
// (effects/counters.go's removal primitives, rules/turn.go:64's suspend TIME
// upkeep decrement), filtered by CounterType$ (case-insensitive, same as the
// Added arm), ValidCard$/ValidPlayer$ and TriggerZones$ (the shared zoneGate
// already ran). CounterChange carries no player field, so ValidPlayer$ is
// matched against the object's controller -- the convention counterAddedMatches
// uses. NewCounterAmount$ is Forge's "the LAST counter" gate: the post-event
// total for the counter kind must equal the named value -- events.Apply folds
// the removal before checkTriggers runs, so o.Counter(ev.Counter) is already
// the total after. A malformed value fails closed (the Added arm's
// strconv/splitCompare style). Fire-once semantics are the event granularity:
// one Amount: -N batch removal is ONE trigger, exactly as CounterAdded fires
// once per CounterChange, and that IS the CounterRemovedOnce contract --
// "one or more counters removed" is one CounterChange, never one trigger per
// counter. The two modes differ only in that a CounterRemovedOnce body reads
// the removed magnitude through TriggerCount$Amount (Chandra, Fire Artisan's
// "deals that much damage"; B.O.B. Bevy of Beebles; Regenerations Restored),
// which the referent capture in trigger_referents.go supplies. No corpus
// CounterRemovedOnce line carries CounterAmount$/NewCounterAmount$.
func (e *Engine) counterRemovedMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if ev.Kind != events.CounterChange || ev.Amount >= 0 {
		return false
	}
	o := e.G.Obj(ev.Obj)
	if o == nil {
		return false
	}
	if kind := t.Params["CounterType"]; kind != "" && !strings.EqualFold(kind, ev.Counter) {
		return false
	}
	if !e.eventCardAndPlayerMatch(t, source, ev.Obj, o.Controller) {
		return false
	}
	if want := t.Params["NewCounterAmount"]; want != "" {
		n, err := strconv.Atoi(strings.TrimSpace(want))
		if err != nil {
			return false
		}
		if o.Counter(ev.Counter) != int32(n) {
			return false
		}
	}
	return true
}

func init() {
	registerTrigMatcher((*Engine).counterAddedMatches, "CounterAdded", "CounterAddedOnce")
	registerTrigMatcher((*Engine).counterRemovedMatches, "CounterRemoved", "CounterRemovedOnce")
}
