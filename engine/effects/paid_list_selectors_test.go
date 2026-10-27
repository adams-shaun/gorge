package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPaidListSelectors pins the cost-paid list spellings (cr-g261) and the
// `AttachedBy <referent>` prefix selector in definedSpec.
func TestPaidListSelectors(t *testing.T) {
	ids := func(ts []state.Target) []state.ObjID {
		var out []state.ObjID
		for _, x := range ts {
			out = append(out, x.Obj)
		}
		return out
	}
	same := func(got []state.ObjID, want ...state.ObjID) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	h := newHost(t, 1)
	c := &Ctx{Source: 99, Controller: 0}
	c.Paid.Exiled = []state.ObjID{11}
	c.Paid.Revealed = []state.ObjID{12}
	c.Paid.Tapped = []state.ObjID{13}
	c.Paid.Returned = []state.ObjID{14}
	c.Sacrificed = []state.SacrificedInfo{{Obj: 15}, {Obj: 16}}
	for spec, want := range map[string][]state.ObjID{
		"Exiled": {11}, "ExiledCards": {11},
		"Revealed": {12}, "RevealedCards": {12},
		"Tapped": {13}, "TappedCards": {13},
		"Returned": {14}, "ReturnedCards": {14},
		"Sacrificed": {15, 16}, "SacrificedCards": {15, 16},
	} {
		got, ok := definedSpec(h, c, spec)
		if !ok || !same(ids(got), want...) {
			t.Errorf("%s = %v ok=%v, want %v", spec, ids(got), ok, want)
		}
	}
	// An empty list names nobody, never the source.
	empty := &Ctx{Source: 99, Controller: 0}
	for _, spec := range []string{"Exiled", "ExiledCards", "RevealedCards", "Tapped", "TappedCards",
		"Returned", "ReturnedCards", "Sacrificed", "SacrificedCards"} {
		got, ok := definedSpec(h, empty, spec)
		if !ok || len(got) != 0 {
			t.Errorf("empty %s = %v ok=%v, want known-empty", spec, ids(got), ok)
		}
	}

	// AttachedBy: a live attachment names its bearer, a departed one its
	// LastBearer, an unattached object nobody, an unknown referent is unknown.
	bh, bc, m := attachedSelectorBoard(t)
	bc.Remembered = []state.Target{{Obj: m["aura"]}}
	if got, ok := definedSpec(bh, bc, "AttachedBy Remembered"); !ok || !same(ids(got), m["bear"]) {
		t.Errorf("AttachedBy Remembered (live Aura) = %v ok=%v, want bearer %d", ids(got), ok, m["bear"])
	}
	bc.Remembered = []state.Target{{Obj: m["graveAura"]}}
	if bh.g.Obj(m["graveAura"]).AttachedTo != 0 {
		t.Fatal("precondition: the departed Aura must be unattached")
	}
	if got, ok := definedSpec(bh, bc, "AttachedBy Remembered"); !ok || !same(ids(got), m["bear"]) {
		t.Errorf("AttachedBy Remembered (departed Aura) = %v ok=%v, want LastBearer %d", ids(got), ok, m["bear"])
	}
	bc.Remembered = []state.Target{{Obj: m["other"]}}
	if got, ok := definedSpec(bh, bc, "AttachedBy Remembered"); !ok || len(got) != 0 {
		t.Errorf("AttachedBy of an unattached object = %v ok=%v, want known-empty", ids(got), ok)
	}
	if got, ok := definedSpec(bh, bc, "AttachedBy NoSuchReferent"); ok {
		t.Errorf("AttachedBy of an unknown referent = %v, want unknown", ids(got))
	}
}
