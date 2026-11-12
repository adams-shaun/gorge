package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

var kwRefSuspend = cards.NewKeywordRef("Suspend")

// hasSuspendAbility reports whether o carries the Suspend ability CR 702.62a
// describes: the printed keyword, the suspend action's provenance, or an
// event-backed grant. A card in exile with it and a time counter is suspended
// (CR 702.62b) however it got there -- the suspend action, or a spell that
// exiles itself with time counters (Rousing Refrain, Inspiring Refrain).
func hasSuspendAbility(o *state.Object) bool {
	return o.CastFlags&state.FlagSuspend != 0 || o.SuspendGranted ||
		(o.Face() != nil && o.Face().HasKeywordRef(kwRefSuspend))
}
