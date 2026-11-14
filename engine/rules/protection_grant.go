package rules

import "github.com/adams-shaun/gorge/engine/state"

// protectionOracle is the slice of the engine protectedFromKW reads.
type protectionOracle interface {
	Keywords(state.ObjID) []string
	sourceHasQuality(source state.ObjID, q string) bool
	controllerOf(state.ObjID) state.PlayerID
}
