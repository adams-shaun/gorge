package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Room's door trigger is that door's ability with its own rules text even
// when the other door is the live one (CR 709.5). Roaring Furnace's front-door
// UnlockDoor trigger reads X:Count$CardsInYourHand, a name only the front half
// defines; cast as Steaming Sauna (the back half live) and then unlocking the
// front door, the trigger must resolve its body against the front door's SVar
// table. Falling back to the live face's table leaves X unresolved and deals 0.
func TestRoomInactiveDoorTriggerReadsItsOwnSVars(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	furnace, ok := reg.Lookup("Roaring Furnace // Steaming Sauna")
	if !ok {
		t.Fatal("corpus fixture: Roaring Furnace // Steaming Sauna missing")
	}
	brontodon, ok := reg.Lookup("Ancient Brontodon")
	if !ok {
		t.Fatal("corpus fixture: Ancient Brontodon missing")
	}
	e := corpusEngine(t, reg, []*cards.Card{furnace}, []*cards.Card{brontodon})

	target := moveByName(t, e, 1, "Ancient Brontodon", state.ZBattlefield)
	id := moveByName(t, e, 0, "Roaring Furnace", state.ZBattlefield)
	// The Room is live as Steaming Sauna: the Roaring Furnace door is locked
	// and inert until the special action unlocks it.
	e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: 1})
	designateRoomCastFace(e, id)
	o := e.G.Obj(id)
	if o == nil || o.FaceIdx != 1 || !o.DoorUnlocked(1) || o.DoorUnlocked(0) {
		t.Fatalf("precondition: Room is not live as Steaming Sauna with only the Roaring Furnace door locked: %+v", o)
	}
	front := o.Card.Faces[0]
	if front.Name != "Roaring Furnace" || front.SVars["X"] == "" {
		t.Fatalf("precondition: the front door must carry the X SVar (name %q, X %q)", front.Name, front.SVars["X"])
	}
	if back := o.Card.Faces[1]; back.SVars["X"] != "" || back.SVars["TrigDamage"] != "" {
		t.Fatalf("precondition: the back door must not define the front door's SVars (TrigDamage %q, X %q)", back.SVars["TrigDamage"], back.SVars["X"])
	}
	want := int32(len(e.G.Zone(state.ZHand, 0)))
	if want == 0 {
		t.Fatal("precondition: the hand count X reads is zero")
	}

	e.emit(events.Event{Kind: events.DoorUnlock, Obj: id})
	if o = e.G.Obj(id); !o.DoorUnlocked(0) || !o.DoorUnlocked(1) {
		t.Fatalf("precondition: the unlock did not unlock both doors: face0=%v face1=%v", o.DoorUnlocked(0), o.DoorUnlocked(1))
	}
	e.priorityRound()
	if len(e.G.Stack) != 1 {
		t.Fatalf("the front door's unlock trigger is not on the stack: %d items", len(e.G.Stack))
	}
	resolveStack(t, e)
	if got := e.G.Obj(target).Damage; got != want {
		t.Fatalf("the front door's unlock trigger dealt %d damage, want the hand count %d (X read from the live face's SVar table)", got, want)
	}
}
