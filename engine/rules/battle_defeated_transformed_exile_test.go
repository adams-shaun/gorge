package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 310.12b: only the removal of a Siege battle's last defense counter
// defeats it. A Siege that was defeated and cast transformed is a creature
// with no defense counters; exiling that creature (Shauku, Endbringer's
// ability) leaves the card as its front-face battle, but it was never
// defeated, so its owner is not offered the transformed cast again.
func TestTransformedSiegeExiledIsNotDefeated(t *testing.T) {
	t.Parallel()
	e, id := defeatedBoard(t, "Invasion of Pyrulea")
	defeatBattle(t, e, id)
	awaitDefeatAsk(t, e)
	submitChoices(t, e, 0)
	for i := 0; i < 12 && e.G.Obj(id).Zone == state.ZStack; i++ {
		passPriority(t, e)
	}
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.FaceIdx != 1 {
		t.Fatalf("precondition: want the transformed back face on the battlefield, got zone %v face %d", o.Zone, o.FaceIdx)
	}
	if got := o.Counter("DEFENSE"); got != 0 {
		t.Fatalf("precondition: the transformed face carries %d defense counters, want 0", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
	if o = e.G.Obj(id); o.Zone != state.ZExile || o.FaceIdx != 0 || !o.Face().IsBattle() {
		t.Fatalf("precondition: want the card exiled as its front-face battle, got zone %v face %d", o.Zone, o.FaceIdx)
	}
	if len(e.defeatedCasts) != 0 {
		t.Fatalf("exiling the transformed creature queued %d defeat-cast offers, want none", len(e.defeatedCasts))
	}
}
