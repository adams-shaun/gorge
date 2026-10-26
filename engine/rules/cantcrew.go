package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// cantCrew reports whether a stat:CantCrew static ("enchanted creature can't
// crew Vehicles", Intercessor's Arrest) forbids creature id from being tapped
// to pay a Crew cost (CR 702.122a). Forge's StaticAbilityCantCrew reads
// ValidCard$ only; the shared restriction gate guards the rest. It affects
// only Crew tap costs -- pay.FilterCrewCandidates is its one consumer, so the
// creature still pays any other tap cost.
func (e *Engine) cantCrew(id state.ObjID) bool {
	for _, sv := range e.activeStatics("CantCrew") {
		if !e.restrictionGateHolds(sv, id) {
			continue
		}
		if e.matchesSpec(sv.ParamStr(cards.PKValidCard), id, e.staticSpecCtx(sv)) {
			return true
		}
	}
	return false
}
