package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 310.11: only a battle the defeat state-based action exiles earns its
// owner the transformed cast. A Siege cast transformed is a nonbattle
// permanent with no defense counters; exiling it (Radiant Purge on a crewed
// Aetherwing) is no defeat and must queue no offer.
func TestExiledBackFaceOfBattleGetsNoCastOffer(t *testing.T) {
	t.Parallel()
	e, id := defeatedBoard(t, "Invasion of Pyrulea")
	defeatBattle(t, e, id)
	if d := awaitDefeatAsk(t, e); d == nil {
		t.Fatal("precondition: the defeat posed no transformed-cast ask")
	}
	submitChoices(t, e, 0)
	for i := 0; i < 12 && e.G.Obj(id).Zone == state.ZStack; i++ {
		passPriority(t, e)
	}
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.FaceIdx != 1 {
		t.Fatalf("precondition: the cast resolved into zone %v face %d, want the battlefield as the back face", o.Zone, o.FaceIdx)
	}
	if got := battleDefenseBefore(o); got == 0 {
		t.Fatal("a transformed battle with no defense counters reads as defeatable")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
	if len(e.defeatedCasts) != 0 {
		t.Fatalf("exiling the back face queued %d cast offers, want none", len(e.defeatedCasts))
	}
}
