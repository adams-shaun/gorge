package rules

import "github.com/adams-shaun/gorge/engine/state"

// sourceControllerReader is the one Engine read triggerTurnController needs.
type sourceControllerReader interface {
	controllerOf(id state.ObjID) state.PlayerID
}

// triggerTurnController is the player whose turn a PlayerTurn$/OpponentTurn$
// trigger gate compares against. When the trigger's source is the very object
// the event moved (lki snapshots it as it last existed), the ability is a
// look-back trigger (CR 603.10a) and its controller is the one it had before
// the move -- a stolen Ketramose, the New Dawn exiled from the battlefield on
// its thief's turn triggers for the thief, not for the owner it reverted to in
// exile. Every other source reads its live controller.
func triggerTurnController(e sourceControllerReader, source state.ObjID, lki *state.Object) state.PlayerID {
	if lki != nil && lki.ID == source {
		return lki.Controller
	}
	return e.controllerOf(source)
}
