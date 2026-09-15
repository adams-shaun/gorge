package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Pair", effPair) }

// effPair implements Soulbond's pairing (CR 702.103): when a Soulbond
// creature enters the battlefield that isn't already paired, its controller
// may pair it with another unpaired creature that can be paired. The
// keyword expands (cards/keywords.go) into a ChangesZone trigger on the
// source's own battlefield entry. This effect does the deterministic engine
// stand-in: it pairs the source with the FIRST eligible unpaired creature its
// controller controls (a real Soulbond is a player's choice, but the choice
// machinery is M4's owner), then reciprocally sets the partner's Paired field
// so both creatures read as paired and any Affected$Paired/PairedWith static
// grants apply. If there is no eligible creature the source simply stays
// unpaired.
//
// Both Paired writes go through events (Event Pair with the two object ids)
// rather than direct field writes, so the pairing is replayed exactly.
func effPair(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	src := g.Obj(c.Source)
	if src == nil || src.Face() == nil || src.Zone != state.ZBattlefield {
		return
	}
	if src.Paired != 0 {
		return // already paired; Soulbond pairs on entry at most once.
	}
	for _, id := range g.Zone(state.ZBattlefield, c.Controller) {
		if id == c.Source {
			continue
		}
		o := g.Obj(id)
		if o == nil || o.Face() == nil || o.Paired != 0 {
			continue
		}
		// A creature can only be paired with a creature it could have been
		// paired with (any other creature per CR 702.103, unless a restriction
		// says otherwise). Do not pair it with itself; pick the first eligible
		// in zone-walk order (the deterministic stand-in).
		h.Emit(events.Event{Kind: events.Pair, Obj: c.Source, IDs: []state.ObjID{id}})
		return
	}
}

