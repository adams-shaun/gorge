package effects

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// AdaptedMark is the events.AlterAttribute Text a resolving Adapt$ put emits
// for the creature that adapted (CR 701.46). It is a pure marker: the fold
// has no case for it, so nothing is stored on the object; the Mode$ Adapt
// trigger matcher (engine/rules/trigmatch/adapt.go) reads it, and Amount
// carries the N the creature adapted with.
const AdaptedMark = "Adapted"

// putCounterAdaptBlocked is the resolve-time half of an Adapt$ put's
// condition (CR 702.35a: "If this creature has no +1/+1 counters on it, put N
// +1/+1 counters on it"): true when the recipient o holds a +1/+1 counter and
// no CanAdapt grant (Biomancer's Familiar's "adapts as though it had no +1/+1
// counters on it") applies to it. For an AB$ activation the offer-time gate
// (engine/rules/legal_activation.go's adaptGateOK) already withheld the
// ability while counters were present, but counters can arrive in response
// between activation and resolution; the effect's own if-condition is also
// the only gate a chained DB$ body (Jetfire's "then adapt 3") ever gets. A
// put that is not an Adapt$ one is never blocked here.
func putCounterAdaptBlocked(h Host, o *state.Object, adapt bool) bool {
	return adapt && o.Counter("P1P1") > 0 && !h.AdaptsAsIfNoCounters(o.ID)
}

// putCounterAdaptMark emits the Adapted marker once a creature's Adapt$ put
// has landed its n counters, after the counters so the trigger sees them. A
// put that is not an Adapt$ one, or whose count resolved to zero (the
// creature put nothing, so did not adapt), emits nothing.
func putCounterAdaptMark(h Host, o *state.Object, adapt bool, n int32) {
	if !adapt || n < 1 {
		return
	}
	h.Emit(events.Event{Kind: events.AlterAttribute, Obj: o.ID,
		Player: o.Controller, Text: AdaptedMark, Amount: n})
}
