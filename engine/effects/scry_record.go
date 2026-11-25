package effects

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ScryRecord builds the completed-scry marker (events.Scry) every emitter
// shares: Player is the scrying seat, Obj the resolving source, Amount the
// number of cards the player put on the BOTTOM (TriggerCount$ScryBottom, the
// ToBottom$ gate), and Pairs[0][0] the number of cards LOOKED at
// (TriggerCount$ScryNum, CR 701.22a: scry N looks at the top N -- after any
// modification of N, clamped to the library). The looked-at count rides an
// existing field (the die-roll Notes' Pairs-as-integers precedent), so no
// events.Kind or field is added.
func ScryRecord(p state.PlayerID, source state.ObjID, looked, bottom int32) events.Event {
	return events.Event{Kind: events.Scry, Player: p, Obj: source, Amount: bottom,
		Pairs: [][2]state.ObjID{{state.ObjID(looked), 0}}}
}

// ScryRecordLooked decodes the number of cards a completed-scry marker
// looked at; 0 for any other event or a marker without the count.
func ScryRecordLooked(ev events.Event) int32 {
	if ev.Kind != events.Scry || len(ev.Pairs) == 0 {
		return 0
	}
	return int32(ev.Pairs[0][0])
}
