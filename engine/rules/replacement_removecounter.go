package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
)

func init() { effects.RegisterNonAPI("repl:RemoveCounter") }

// applyRemoveCounterReplacements is the counter-removal class's pass: the
// event is a negative-amount CounterChange the collection matched to
// R:Event$ RemoveCounter lines, and the class's only corpus shape is
// Layer$ CantHappen (Fear of Sleep Paralysis: "stun counters can't be
// removed from permanents your opponents control"), a prohibition with no
// ReplaceWith$ body. CR 614.1a: the removal does not happen. The counter
// stays and whatever the removal stood in for stays undone -- the untap a
// stun counter is removed for (CR 122.1d, effects.tryUntapBy) does not
// happen either, so the permanent stays tapped. The Note is the prevented
// marker the drawForbidden convention emits; the original CounterChange is
// never logged.
func (e *Engine) applyRemoveCounterReplacements(ev events.Event, matches []replMatch) (events.Event, bool) {
	if len(matches) == 0 {
		return ev, false
	}
	// CR 616.1: a CantHappen prohibition is absolute -- no affected player
	// chooses, and matching lines agree on "not at all", so one prevented
	// Note covers the whole set.
	return e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
		Text: "prevented: counter removal cannot happen"}), true
}
