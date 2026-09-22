package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// TestNumResolvedReadsDirectRefProperty pins the direct numeric-parameter route
// used by Flying Kick: NumDmg$ ParentTargeted$CardPower must evaluate the
// selected parent object rather than falling through to zero.
func TestNumResolvedReadsDirectRefProperty(t *testing.T) {
	h := newHost(t, 2)
	parentCard := mkCard(t, "Name:Parent\nTypes:Creature\nPT:5/7\nOracle:x\n")
	otherCard := mkCard(t, "Name:Other\nTypes:Creature\nPT:1/1\nOracle:x\n")
	parent := h.g.AddObject(parentCard, 0)
	other := h.g.AddObject(otherCard, 1)
	parentID, otherID := parent.ID, other.ID
	c := &Ctx{Source: parentID, Controller: 0, Targets: []state.Target{{Obj: parentID}}}
	sa := &cards.SA{Params: map[string]string{"NumDmg": "ParentTargeted$CardPower"}}
	if h.g.Obj(parentID).Zone != state.ZLibrary || h.g.Obj(otherID).Zone != state.ZLibrary {
		t.Fatalf("precondition: test objects must be in the host's library zone: %s, %s", h.g.Obj(parentID).Zone, h.g.Obj(otherID).Zone)
	}
	if got := h.g.Obj(parentID).Face().Power(); got == h.g.Obj(otherID).Face().Power() {
		t.Fatalf("precondition: parent and other powers must differ: %d and %d", got, h.g.Obj(otherID).Face().Power())
	}
	if n, ok := NumResolved(h, c, sa, "NumDmg", 0); !ok || n != 5 {
		t.Fatalf("direct ParentTargeted$CardPower = (%d, %v), want (5, true)", n, ok)
	}

	unknown := &cards.SA{Params: map[string]string{"NumDmg": "ParentTargeted$UnmodelledProperty"}}
	if _, ok := NumResolved(h, c, unknown, "NumDmg", 0); ok {
		t.Fatal("unknown direct ref/property reported evaluated")
	}
}
