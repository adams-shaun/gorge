package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// handCard adds a card named name to owner's hand and returns its id.
func handCard(t *testing.T, h *fakeHost, name string, owner state.PlayerID) state.ObjID {
	t.Helper()
	o := h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:Creature\nPT:1/1\nOracle:x\n"), owner)
	o.Zone = state.ZHand
	return o.ID
}
