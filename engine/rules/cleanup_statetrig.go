package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// stateTriggerProber is the one engine capability checkStateTriggersAfterCleanup
// needs: the trigger walk an emitted event runs.
type stateTriggerProber interface {
	checkTriggers(ev *events.Event, lki *state.Object, lkiPower, lkiToughness int32, lkiPTValid bool)
}

// checkStateTriggersAfterCleanup runs the CR 603.8 state-trigger check after
// the CR 514.2 cleanup actions. Ending an "until end of turn" effect
// (EndOfTurnCleanup) changes the game state without emitting an event, and a
// Mode$ Always trigger is matched only from an event's trigger walk. Without
// this check, a state trigger whose condition the expiry makes true waits for
// the next turn's untap events. Example: Kukemssa Serpent's "when you control
// no Islands" after a land that was an Island until end of turn reverts. CR
// 514.3a needs it to trigger in this cleanup step and give priority. The probe
// is an unlogged Note with no amount or text: Always's matcher holds for any
// event, and the one Note-reading matcher (DamagePrevented) needs a positive
// amount. The walk only queues triggers. Nothing is written to the log or the
// game here.
func checkStateTriggersAfterCleanup(p stateTriggerProber) {
	probe := events.Event{Kind: events.Note}
	p.checkTriggers(&probe, nil, 0, 0, false)
}
