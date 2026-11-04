package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Vehicle that became a copy of an Equipment until end of turn and was
// equipped to a creature in that window is no Equipment once the copy ends at
// cleanup, so CR 704.5n unattaches it there, and the unattach is a
// state-based action performed in the cleanup step (CR 514.3a).
func TestEndedEquipmentCopyUnattachesAtCleanup(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := searchEngine(t, reg, "Shuri, Wakandan Inventor", "S.H.I.E.L.D. Flying Car", "Bonesplitter", "Grizzly Bears")
	shuri := searchMoveByName(t, e, "Shuri, Wakandan Inventor", state.ZBattlefield)
	car := searchMoveByName(t, e, "S.H.I.E.L.D. Flying Car", state.ZBattlefield)
	splitter := searchMoveByName(t, e, "Bonesplitter", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	face := e.G.Obj(shuri).Face()
	effects.Resolve(e, &effects.Ctx{Source: shuri, Controller: 0,
		Targets:       []state.Target{{Obj: car}},
		PickedTargets: []state.Target{{Obj: splitter}},
		SVars:         face.SVars}, cards.ResolveSVar(face.SVars, "DBClone"))
	if !e.isEquipment(e.G.Obj(car)) {
		t.Fatalf("precondition: the Vehicle did not become a copy of the Equipment (%s)", e.G.Obj(car).Face().Name)
	}
	e.emit(events.Event{Kind: events.Attach, Obj: car, IDs: []state.ObjID{bear}})
	e.checkStateBased()
	if e.G.Obj(car).AttachedTo != bear {
		t.Fatal("precondition: the copy did not stay attached")
	}
	e.EndOfTurnCleanup()
	before := len(e.L.Events)
	e.checkStateBased()
	if e.isEquipment(e.G.Obj(car)) {
		t.Fatal("the copy outlived the cleanup step")
	}
	if got := e.G.Obj(car).AttachedTo; got != 0 {
		t.Fatalf("the Vehicle is still attached to %d after its copy ended", got)
	}
	if len(e.L.Events) == before {
		t.Fatal("the unattach was not a logged state-based action")
	}
}
