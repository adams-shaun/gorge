package events

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/state"
)

// noteStackHighWater raises g.MaxDistinctOnStackThisTurn to the number of
// distinct sources now on the stack (CR 405), when that is a new high. A spell
// is its own source; an ability's source is its Source, so a spell and its own
// ability, or two abilities of one permanent, count once. An ability with no
// Source (the Ring emblem's) is its own source.
func noteStackHighWater(g *state.Game) {
	keys := make([]state.ObjID, 0, len(g.Stack))
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		if o.Card == nil && o.Source != 0 {
			keys = append(keys, o.Source)
		} else {
			keys = append(keys, id)
		}
	}
	slices.Sort(keys)
	if n := int32(len(slices.Compact(keys))); n > g.MaxDistinctOnStackThisTurn {
		g.MaxDistinctOnStackThisTurn = n
	}
}
