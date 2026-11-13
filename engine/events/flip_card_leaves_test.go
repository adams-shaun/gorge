package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestFlipCardLeavesBattlefieldUnflipped pins CR 710.4 / 710.2: a flipped
// flip card that leaves the battlefield retains no memory of its status and
// has only its normal characteristics in its next zone.
func TestFlipCardLeavesBattlefieldUnflipped(t *testing.T) {
	g, l := twoPlayer(t)
	o := g.AddObject(dfcFixture(t, "Flip"), 0)
	o.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), o.ID))
	Emit(g, l, Event{Kind: FlipFace, Obj: o.ID, Amount: 1})
	if o.FaceIdx != 1 || o.Face().Name != "Back" {
		t.Fatalf("precondition: FaceIdx=%d name=%q, want the flipped half on the battlefield", o.FaceIdx, o.Face().Name)
	}
	Emit(g, l, Event{Kind: MoveZone, Obj: o.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	if o.Zone != state.ZGraveyard || o.FaceIdx != 0 || o.Face().Name != "Front" {
		t.Fatalf("zone %v FaceIdx=%d name=%q, want the unflipped card in the graveyard", o.Zone, o.FaceIdx, o.Face().Name)
	}
}
