package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTransformedBattleExiledGetsNoCastOffer: a Siege whose back face is on the
// battlefield is that back-face permanent (CR 712.8), never a battle, so a spell
// that exiles it (Excise the Imperfect) defeats nothing (CR 310.11): neither
// the counter removal nor the exile puts a Siege defeat trigger (CR 310.12b)
// on the stack, so no transformed-cast offer follows.
func TestTransformedBattleExiledGetsNoCastOffer(t *testing.T) {
	t.Parallel()
	e, id := defeatedBoard(t, "Invasion of Pyrulea")
	e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: 1, Text: "Transformed"})
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "DEFENSE", Amount: -e.G.Obj(id).Counter("DEFENSE")})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.FaceIdx != 1 {
		t.Fatalf("precondition: want the transformed battle on the battlefield, got %+v", o)
	}
	if o.Face().IsBattle() || o.Counter("DEFENSE") != 0 {
		t.Fatalf("precondition: the back face must be a non-battle with no defense counters")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("exiling the transformed Siege queued %d triggers, want no defeat trigger", len(e.pendingTriggers))
	}
}
