package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A spell's card leaves its zone for the stack at CR 601.2a, before its
// targets (601.2c) and its costs (601.2h). The cast triggers wait for 601.2i
// (emit holds the PutOnStack back), but a zone-change trigger the move itself
// sets off -- Hologram Stalker's or Vash, Freelance Archaeologist's "whenever a
// card leaves your graveyard" -- triggers at the move, on the board as it is
// then (CR 603.2). So a beam me up or flashback cast whose cost returns or
// sacrifices the watcher still triggers it, and the ability goes on the stack
// though its source has left (CR 603.3, 113.7a).
const (
	pushZoneOnly uint8 = 1 + iota // the push: zone-change modes only
	pushZoneDone                  // the deferred re-walk: every mode but those
)

// pushZoneMode reports whether a trigger mode is one the push walks.
func pushZoneMode(t *cards.Trigger) bool {
	m := t.ModeKind()
	return m == cards.TriggerChangesZone || m == cards.TriggerChangesZoneAll
}

// pushZoneExcludes is the mode filter triggerMatches applies while one of
// the two PutOnStack walks runs.
func (e *Engine) pushZoneExcludes(t *cards.Trigger, ev *events.Event) bool {
	if e.pushZoneWalk == 0 || ev.Kind != events.PutOnStack {
		return false
	}
	return pushZoneMode(t) != (e.pushZoneWalk == pushZoneOnly)
}

// checkPushZoneTriggers walks the zone-change triggers of a spell's up-front
// PutOnStack as it happens. Their queue entries are the proposal's: a cast
// that is reversed drops them and puts back the trigger limits they used
// (CR 733.1: no ability triggers as a result of an undone action).
func (e *Engine) checkPushZoneTriggers(ev events.Event, lki *state.Object) {
	pc := e.cast
	if pc == nil || pc.isAbility() || ev.Kind != events.PutOnStack {
		return
	}
	before := len(e.pendingTriggers)
	preTurn, preGame := e.limitSnapshot()
	e.pushZoneWalk = pushZoneOnly
	e.checkFaceTriggers(e, &ev, lki, 0, 0, false, false, false)
	e.pushZoneWalk = 0
	if len(e.pendingTriggers) > before {
		pc.proposalLimits = append(pc.proposalLimits, e.limitChanges(preTurn, preGame)...)
		pc.proposalTriggers = append(pc.proposalTriggers, [2]int{before, len(e.pendingTriggers)})
	}
}
