package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestExileTurnFaceUpFoldsOnlyAFaceDownExiledCard pins the fold: a face-down
// card in exile turns face up with its MayLook/ExiledWith provenance intact,
// while a face-down battlefield object and a face-up exiled card are untouched.
func TestExileTurnFaceUpFoldsOnlyAFaceDownExiledCard(t *testing.T) {
	mk := func(zone state.Zone, faceDown bool) (*state.Game, *state.Object) {
		g, id := gameWithOneCard(t)
		o := g.Obj(id)
		moveObj := func() {
			g.SetZone(state.ZHand, 0, nil)
			g.SetZone(zone, 0, []state.ObjID{id})
			o.Zone = zone
		}
		moveObj()
		o.FaceDown = faceDown
		o.MayLookPlayer = 1
		o.ExiledWith = 77
		return g, o
	}

	g, o := mk(state.ZExile, true)
	if o.Zone != state.ZExile || !o.FaceDown {
		t.Fatalf("precondition: zone=%v faceDown=%v", o.Zone, o.FaceDown)
	}
	Apply(g, Event{Kind: ExileTurnFaceUp, Obj: o.ID})
	if o.FaceDown {
		t.Fatalf("face-down exiled card stayed face down")
	}
	if o.Zone != state.ZExile || o.MayLookPlayer != 1 || o.ExiledWith != 77 {
		t.Fatalf("provenance disturbed: zone=%v mayLook=%v exiledWith=%v", o.Zone, o.MayLookPlayer, o.ExiledWith)
	}

	g, o = mk(state.ZBattlefield, true)
	Apply(g, Event{Kind: ExileTurnFaceUp, Obj: o.ID})
	if !o.FaceDown {
		t.Fatalf("face-down battlefield object was turned up by ExileTurnFaceUp")
	}

	g, o = mk(state.ZExile, false)
	Apply(g, Event{Kind: ExileTurnFaceUp, Obj: o.ID})
	if o.FaceDown || o.Zone != state.ZExile || o.ExiledWith != 77 {
		t.Fatalf("face-up exiled card changed: %+v", o)
	}
}
