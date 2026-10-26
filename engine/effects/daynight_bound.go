package effects

import "github.com/adams-shaun/gorge/engine/state"

// dayNightBound reports whether id has daybound or nightbound as a derived
// keyword. CR 702.145b/e: such a permanent can't transform except by its own
// daybound/nightbound ability, so SetState Mode$ Transform leaves it alone.
func dayNightBound(h Host, id state.ObjID) bool {
	return h.HasKeyword(id, "Daybound") || h.HasKeyword(id, "Nightbound")
}
