package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.26b with CR 701.54a: a phased-out creature is treated as though it
// doesn't exist, so the Ring's bearer choice never offers it.
func TestRingTemptsYouSkipsPhasedOutCreatures(t *testing.T) {
	h := newHost(t, 2)
	bear := ringCreature(t, h, 0, "Bear")
	out := ringCreature(t, h, 0, "Efreet")
	gorilla := ringCreature(t, h, 0, "Gorilla")
	h.g.Obj(out).PhasedOut = true
	h.askResult = true

	Resolve(h, &Ctx{Controller: 0}, sa(t, "DB$ RingTemptsYou"))

	if h.askCount != 1 || h.lastAsk == nil {
		t.Fatalf("precondition: two eligible creatures must pose the ask, count %d", h.askCount)
	}
	var offered []state.ObjID
	for _, o := range h.lastAsk.Options {
		offered = append(offered, o.Obj)
	}
	if len(offered) != 2 || offered[0] != bear || offered[1] != gorilla {
		t.Fatalf("offered %v, want [%d %d] without the phased-out %d", offered, bear, gorilla, out)
	}
}
