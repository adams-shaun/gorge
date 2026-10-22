package policynet

import (
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// entTarget is one object target of one stack entry: the targeted object
// and whether the entry's controller is the encoding seat.
type entTarget struct {
	obj  state.ObjID
	mine bool
}

// entPair is one attacker/blocker relation (ENTREL-B fills it).
type entPair struct {
	att, blk state.ObjID
}

// stackHas reports whether id is a stack object.
func stackHas(v *view.View, id state.ObjID) bool {
	for i := range v.Stack {
		if v.Stack[i].ID == id {
			return true
		}
	}
	return false
}

// graveyardHas reports whether id is in some player's graveyard.
func graveyardHas(v *view.View, id state.ObjID) bool {
	for i := range v.Players {
		for j := range v.Players[i].Graveyard {
			if v.Players[i].Graveyard[j].ID == id {
				return true
			}
		}
	}
	return false
}

// entityRelations writes the relational slots (EntityRawWidthV1..) of one
// listed card: how many stack entries target it, and for a listed stack card
// what its own entry targets, its X and whether it was kicked.
func entityRelations(raw []float32, cv *view.CardView, zone int, sv *view.StackView, seat state.PlayerID, b *entityBoard) {
	if len(b.tgts) == 0 && sv == nil && len(b.pairs) == 0 {
		return
	}
	for _, t := range b.tgts {
		if t.obj != cv.ID {
			continue
		}
		if t.mine {
			raw[entTgtByMe]++
		} else {
			raw[entTgtByOpp]++
		}
	}
	if zone != entZoneStack || sv == nil {
		return
	}
	for _, t := range sv.Targets {
		if t.IsPlayer {
			if t.Player == seat {
				raw[entTgtPlayerMe]++
			} else {
				raw[entTgtPlayerOpp]++
			}
			continue
		}
		if t.Obj == 0 {
			continue
		}
		if c, ok := b.controller(t.Obj); ok {
			if c == seat {
				raw[entTgtPermMe]++
			} else {
				raw[entTgtPermOpp]++
			}
			continue
		}
		if stackHas(b.v, t.Obj) {
			raw[entTgtStack]++
			continue
		}
		if graveyardHas(b.v, t.Obj) {
			raw[entTgtGrave]++
		}
	}
	raw[entStackX] = float32(sv.X) / 5
	if sv.Kicks > 0 {
		raw[entStackKicked] = 1
	}
}
