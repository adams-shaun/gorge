package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Greenhouse // Rickety Gazebo cast as Rickety Gazebo, then Greenhouse
// unlocked: the unlocked door's static `AddAbility$ AnyMana` names an SVar that
// only Greenhouse's face defines (CR 709.5), so the grant body must be read
// from that door's own table, not the shown face's.
func TestRoomUnlockedDoorStaticGrantReadsItsTable(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	gh := searchCorpusCard(t, reg, "Greenhouse")
	forest := searchCorpusCard(t, reg, "Forest")
	e := corpusEngine(t, reg, []*cards.Card{gh, forest, forest, forest}, nil)
	id := moveByName(t, e, 0, "Greenhouse", state.ZBattlefield)
	land := moveByName(t, e, 0, "Forest", state.ZBattlefield)
	e.G.Obj(id).SetFaceIdx(1 - e.G.Obj(id).FaceIdx)
	o := e.G.Obj(id)
	if o.Face().Name != "Rickety Gazebo" || o.Face().SVars["AnyMana"] != "" {
		t.Fatalf("precondition: shown face is %q with an AnyMana table", o.Face().Name)
	}
	designateRoomCastFace(e, id)
	before := len(e.grantedAbilities(0, land))
	e.emit(events.Event{Kind: events.DoorUnlock, Obj: id})
	if !e.G.Obj(id).RoomOtherDoorUnlocked() {
		t.Fatal("precondition: Greenhouse door not unlocked")
	}
	if got := len(e.grantedAbilities(0, land)); got != before+1 {
		t.Fatalf("land granted abilities %d -> %d, want one more (any-colour mana)", before, got)
	}
}
