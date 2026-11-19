package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { effects.RegisterNonAPI("repl:RemoveCounter") }

// removeCounterReplacementMatches is the matcher arm of the counter-removal
// class (Fear of Sleep Paralysis's stun-counter lock). Only a
// negative-amount object CounterChange is a removal -- the sign guard
// mirrors the AddCounter class's, and the engine's own status markers
// riding a CounterChange are excluded with it. The class's corpus shape is
// Layer$ CantHappen only (the cards layer keeps Deification's ReplaceWith$
// damage arm on repl:RemoveCounter.OtherShape), and the object form carries
// no player, so a ValidPlayer$-scoped line is unmodelled: fail closed.
func (e *Engine) removeCounterReplacementMatches(r cards.Repl, ev events.Event,
	you state.PlayerID, source state.ObjID, svars map[string]string) bool {
	if ev.Kind != events.CounterChange || ev.Amount >= 0 {
		return false
	}
	if state.InternalCounterMarker(ev.Counter) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(r.ParamStr(cards.PKLayer)), "CantHappen") {
		return false
	}
	if r.With != nil {
		return false
	}
	if ct := strings.TrimSpace(r.ParamStr(cards.PKValidCounterType)); ct != "" && ct != ev.Counter {
		return false
	}
	spec := strings.TrimSpace(r.ParamStr(cards.PKValidCard))
	if spec == "" {
		spec = strings.TrimSpace(r.ParamStr(cards.PKValidObject))
	}
	if spec != "" && !e.matchesSpecFrom(spec, ev.Obj, you, source) {
		return false
	}
	if _, ok := r.Param(cards.PKValidPlayer); ok {
		return false
	}
	return e.replacementConditionHoldsWith(r, source, you, svars)
}

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
