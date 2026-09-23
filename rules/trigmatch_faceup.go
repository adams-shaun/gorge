// Mode$ TurnFaceUp: "When [this/that permanent] is turned face up"
// (CR 708.6 for manifest/cloak, CR 702.36e for morph/megamorph/disguise).
//
// The turned permanent is the event's Obj (events.TurnFaceUp). ValidCard$
// scopes which turned object fires THIS trigger -- Card.Self for a
// self-turn-up body (Printlifter Ooze, Woolly Loxodon), a type/control
// predicate for "whenever a face-down creature you control is turned face
// up". ValidPlayer$ is the turned permanent's controller, read through the
// shared action-trigger clause (eventCardAndPlayerMatch), exactly as the
// other action modes read it.
//
// The turn-up event is emitted by effects.effSetState's Mode$ TurnFaceUp arm,
// the effect-driven route the corpus's `AB$ SetState | Mode$ TurnFaceUp`
// lines carry. The morph/disguise special action that would also emit it is a
// separate subsystem; this matcher is correct for whatever emits the event.
package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// turnFaceUpMatches is Mode$ TurnFaceUp. A turn-up event whose object has
// already left the battlefield, or an event with no object, matches nothing.
func (e *Engine) turnFaceUpMatches(t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.TurnFaceUp || ev.Obj == 0 {
		return false
	}
	o := e.G.Obj(ev.Obj)
	if o == nil {
		return false
	}
	return e.eventCardAndPlayerMatch(t, source, ev.Obj, o.Controller)
}

func init() {
	registerTrigMatcher((*Engine).turnFaceUpMatches, "TurnFaceUp")
}
