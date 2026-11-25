package rules

import "github.com/adams-shaun/gorge/engine/state"

// QueueInitiativeVenture implements effects.Host for CR 726.2: "Whenever a
// player takes the initiative, that player ventures into Undercity" is an
// inherent TRIGGERED ability, so a TakeInitiative resolution only moves the
// designation and this queues the venture for the next priority window.
//
// The entry is the ordinary source-less pendingTrigger: pushTrigger mints its
// synthetic venture body through delayedPushPayload, exactly as the combat
// damage and CR 726.4 handoff takes do (engine/rules/combat.go,
// engine/rules/cantlose.go). CR 726.5's retake case still ventures; the
// InitiativeChange fold is idempotent, and the trigger is queued regardless
// of whether p already held the designation.
func (e *Engine) QueueInitiativeVenture(p state.PlayerID) {
	if p < 0 || int(p) >= len(e.G.Players) || e.G.Players[p].Lost {
		return
	}
	e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
		Controller: p, InitiativeVenture: true,
	})
}
