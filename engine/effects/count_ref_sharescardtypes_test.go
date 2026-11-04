package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestRefSharesCardTypes: <Ref>$SharesCardTypes is the most referenced cards
// that have one card type in common, at least 1, so it equals the
// reference's Amount exactly when every card shares a type (CR 205.2a): an
// artifact creature, an artifact and a creature share no one type though
// each pair does.
func TestRefSharesCardTypes(t *testing.T) {
	h := newHost(t, 2)
	card := func(name, types string) state.ObjID {
		o := h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:"+types+"\nOracle:x\n"), 0)
		o.Zone = state.ZExile
		return o.ID
	}
	golem := card("Brass Walker", "Artifact Creature Golem")
	relic := card("Old Relic", "Artifact")
	bear := card("Field Bear", "Creature Bear")
	marsh := card("Sunk Marsh", "Land")
	for _, tc := range []struct {
		name string
		objs []state.ObjID
		want int32
	}{
		{"none", nil, 1},
		{"one", []state.ObjID{marsh}, 1},
		{"artifacts", []state.ObjID{golem, relic}, 2},
		{"no common type", []state.ObjID{golem, relic, bear}, 2},
		{"creatures", []state.ObjID{golem, bear}, 2},
		{"disjoint", []state.ObjID{relic, marsh}, 1},
	} {
		var rem []state.Target
		for _, id := range tc.objs {
			rem = append(rem, state.Target{Obj: id})
		}
		c := &Ctx{Controller: 0, Remembered: rem}
		if got, ok := EvalCountOK(h, c, "Remembered$SharesCardTypes"); !ok || got != tc.want {
			t.Errorf("%s: Remembered$SharesCardTypes = (%d, %v), want (%d, true)", tc.name, got, ok, tc.want)
		}
	}
}
