package effects

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// beginEntryTappedBatch opens the zone batch that holds a battlefield entry's
// CR 603.6b trigger check until the effect-side Tapped$ rider has set the
// permanent's entry state. The caller closes it with EndZoneBatch after the
// Tap and, when the script carries one, the attacking rider. A non-battlefield
// destination or an untapped entry is a no-op, so a mover can call it
// unconditionally and close only when it reports true.
func beginEntryTappedBatch(h Host, to state.Zone, tapped bool) bool {
	if to != state.ZBattlefield || !tapped {
		return false
	}
	h.BeginZoneBatch()
	return true
}

// emitEntryTapped emits the Tapped$ entry-state Tap (CR 110.5) for an
// effect-side move. It is not the CR 701.21a event of becoming tapped; the
// Text distinguishes it in the replayed log, as every other Tapped$ mover
// does.
func emitEntryTapped(h Host, to state.Zone, tapped bool, id state.ObjID, player state.PlayerID) {
	if to != state.ZBattlefield || !tapped {
		return
	}
	h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: player, Text: "entered tapped"})
}

// closeEntryTappedBatch emits the Tapped$ entry Tap (if any) and closes the
// batch beginEntryTappedBatch opened. The pair is the whole entry-state tail
// of an effect-side mover, called once per moved object.
func closeEntryTappedBatch(h Host, opened bool, to state.Zone, tapped bool, id state.ObjID, player state.PlayerID) {
	emitEntryTapped(h, to, tapped, id, player)
	if opened {
		h.EndZoneBatch()
	}
}

// endEntryTappedBatch closes a batch beginEntryTappedBatch opened. It is the
// no-Tap tail for a mover that emits the entry Tap from its own loop.
func endEntryTappedBatch(h Host, opened bool) {
	if opened {
		h.EndZoneBatch()
	}
}
