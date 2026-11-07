package events

import "github.com/adams-shaun/gorge/engine/state"

// libSlot is 1 + o's index in its owner's library (0: not in it), the place
// a CR 733.1 reversal of a cast from that library puts the card back.
func libSlot(g *state.Game, o *state.Object) int {
	for i, id := range g.Zone(state.ZLibrary, zoneOwner(o, state.ZLibrary)) {
		if id == o.ID {
			return i + 1
		}
	}
	return 0
}

// restoreLibSlot moves o, just returned to its owner's library by a reversed
// cast (CR 733.1-2: the spell returns to the zone it came from as if the cast
// never began), from wherever the move put it to index at, clamped to the
// library. A fresh slice: zone slices may be shared with a clone.
func restoreLibSlot(g *state.Game, o *state.Object, at int) {
	p := zoneOwner(o, state.ZLibrary)
	lib := g.Zone(state.ZLibrary, p)
	out := make([]state.ObjID, 0, len(lib))
	for _, id := range lib {
		if id != o.ID {
			out = append(out, id)
		}
	}
	if len(out) == len(lib) {
		return
	}
	at = min(max(at, 0), len(out))
	out = append(out[:at], append([]state.ObjID{o.ID}, out[at:]...)...)
	g.SetZone(state.ZLibrary, p, out)
}
