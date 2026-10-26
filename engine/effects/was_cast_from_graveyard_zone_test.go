package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestObjectWasCastFromGraveyardReadsCastZone: a may-play graveyard cast
// sets no keyword flag, but the latest PutOnStack zone is the graveyard
// (CR 601.2a), so the one object-aware read holds; a copy and a hand cast do
// not.
func TestObjectWasCastFromGraveyardReadsCastZone(t *testing.T) {
	mk := func(from state.Zone, copyOf bool) *state.Object {
		o := &state.Object{HasLatestCast: true, LatestCastFrom: from, IsCopy: copyOf}
		if o.CastFlags != 0 {
			t.Fatalf("precondition: CastFlags %#x, want none", o.CastFlags)
		}
		return o
	}
	if !state.ObjectWasCastFromGraveyard(mk(state.ZGraveyard, false)) {
		t.Error("a graveyard-zone cast without keyword flags must read true")
	}
	if state.ObjectWasCastFromGraveyard(mk(state.ZGraveyard, true)) {
		t.Error("a stack copy must read false")
	}
	if state.ObjectWasCastFromGraveyard(mk(state.ZHand, false)) {
		t.Error("a hand cast must read false")
	}
	if state.ObjectWasCastFromGraveyard(&state.Object{LatestCastFrom: state.ZGraveyard}) {
		t.Error("a zone without HasLatestCast must read false")
	}
}
