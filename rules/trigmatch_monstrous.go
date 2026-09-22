// The BecomeMonstrous trigger mode.
//
// Mode$ BecomeMonstrous (task kw-monstrosity): "When Stormbreath Dragon
// becomes monstrous, it deals damage to each opponent equal to the number
// of cards in that player's hand." Split out of trigger_match.go so tickets
// touching different modes stop colliding on one file. Registration is at
// the bottom; a duplicate mode panics (registerTrigMatcher).
package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// becomeMonstrousMatches implements Mode$ BecomeMonstrous. The carrier event
// is the events.AlterAttribute "Monstrous" grant the AB$ PutCounter
// Monstrosity$ arm (effects/counters.go) emits after its counters land -- the
// same AlterAttribute fold the plot ACTION and the api:AlterAttribute effect
// use, Text-discriminated. The Kind's ordinal sits past triggerMaskKindBits
// (the Enlisted shape), so triggerModeEvents returns 0 and allows() fails
// open for it; this full matcher is the gate. ValidCard$ is read against the
// marked object exactly as counterAddedMatches reads its CounterChange event,
// with the object's controller as the event player (TriggerZones$ -- the
// Stormbreath shape's `TriggerZones$ Battlefield` -- is the shared zoneGate's
// read, already run). Only a GRANT (Amount >= 1) fires; there is no removal
// spelling in the corpus, and the designation's only clear is the Move
// departure fold, which is not an event to match.
func (e *Engine) becomeMonstrousMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if ev.Kind != events.AlterAttribute || !strings.EqualFold(ev.Text, "Monstrous") || ev.Amount <= 0 {
		return false
	}
	o := e.G.Obj(ev.Obj)
	if o == nil {
		return false
	}
	return e.eventCardAndPlayerMatch(t, source, ev.Obj, o.Controller)
}

func init() {
	registerTrigMatcher((*Engine).becomeMonstrousMatches, "BecomeMonstrous")
}
