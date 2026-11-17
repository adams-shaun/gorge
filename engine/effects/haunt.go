package effects

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// emitHaunt is the Haunt$ True tail of a ChangeZone resolution (CR 702.55b):
// the card just exiled haunts the creature the trigger announced as its
// target. The exile is the ChangeZone's own MoveZone; this adds the link
// (events.Haunt), which Apply folds into state.Object.Haunting. A card that
// did not land in exile, or a missing/illegal creature target, links nothing.
func emitHaunt(h Host, c *Ctx, moved []state.ObjID) {
	g := h.Game()
	var creature state.ObjID
	for _, t := range c.Targets {
		if o := g.Obj(t.Obj); !t.IsPlayer && o != nil && o.Zone == state.ZBattlefield {
			creature = t.Obj
			break
		}
	}
	if creature == 0 {
		return
	}
	for _, id := range moved {
		if o := g.Obj(id); o != nil && o.Zone == state.ZExile {
			h.Emit(events.Event{Kind: events.Haunt, Obj: id, Player: c.Controller, IDs: []state.ObjID{creature}})
		}
	}
}
