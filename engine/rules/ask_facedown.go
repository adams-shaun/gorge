package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// hideFaceDownNames takes the name of every face-down spell or permanent the
// seat asked doesn't control out of d's prompt and option labels (CR 708.5).
// Those texts are built from Face().Name all over the engine, for a seat that
// may be anyone, and ask is the one place they all pass through. A priority
// offer is about the seat's own cards and objects, so it is left alone.
func (e *Engine) hideFaceDownNames(d *decision.Decision) {
	if d.Kind == decision.KPriority {
		return
	}
	g := e.G
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			e.hideFaceDownName(d, id, "a face-down permanent")
		}
	}
	for _, id := range g.Stack {
		e.hideFaceDownName(d, id, "a face-down spell")
	}
}

func (e *Engine) hideFaceDownName(d *decision.Decision, id state.ObjID, shown string) {
	o := e.G.Obj(id)
	if !o.FaceDown || o.Controller == d.Player {
		return
	}
	f := o.Face()
	if f == nil || f.Name == "" {
		return
	}
	d.HideName(id, f.Name, shown, func(other state.ObjID) bool {
		x := e.G.Obj(other)
		if x == nil || x.FaceDown {
			return false
		}
		xf := x.Face()
		return xf != nil && xf.Name == f.Name
	})
}
