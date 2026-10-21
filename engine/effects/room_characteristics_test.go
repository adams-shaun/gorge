package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRoomCharacteristicsFollowDoors is the effects tier's CR 709.5 gate:
// name comparisons, mana value and the roomNames helper follow
// the door designations on the battlefield (none, one, both unlocked; a
// DoorLock narrows them again), and every helper declines off the battlefield
// so the card keeps its printed (combined, CR 709.4) characteristics.
func TestRoomCharacteristicsFollowDoors(t *testing.T) {
	h := newHost(t, 2)
	room := testRoomDoorCard(t, "Front Door", "Back Door")
	id := h.g.AddObject(room, 0).ID
	o := h.g.Obj(id)
	o.Zone = state.ZBattlefield

	if o.DoorUnlocked(0) || o.DoorUnlocked(1) {
		t.Fatalf("precondition: a directly placed Room has unlocked doors: face0=%v face1=%v", o.DoorUnlocked(0), o.DoorUnlocked(1))
	}
	if names, room := roomNames(o); !room || len(names) != 0 {
		t.Fatalf("both-locked roomNames = %v room=%v, want no names on a governed object", names, room)
	}
	if got := objectManaValue(h.g, o); got != 0 {
		t.Fatalf("both-locked objectManaValue = %d, want 0", got)
	}
	if got := nameCharacteristics(o); len(got) != 0 {
		t.Fatalf("both-locked nameCharacteristics = %v, want none", got)
	}
	if sharesName(o, "Front Door", SpecContext{}) || sharesName(o, "Back Door", SpecContext{}) {
		t.Fatal("a both-locked Room shares a name with one of its doors")
	}

	// Cast the front door: only it is unlocked.
	designateRoomCastFace(o)
	if !o.DoorUnlocked(0) || o.DoorUnlocked(1) {
		t.Fatalf("precondition: cast designation left face0=%v face1=%v", o.DoorUnlocked(0), o.DoorUnlocked(1))
	}
	if names, _ := roomNames(o); !slices.Equal(names, []string{"Front Door"}) {
		t.Fatalf("front-door roomNames = %v, want [Front Door]", names)
	}
	if got := objectManaValue(h.g, o); got != 2 {
		t.Fatalf("front-door objectManaValue = %d, want 2", got)
	}
	if !sharesName(o, "Front Door", SpecContext{}) || sharesName(o, "Back Door", SpecContext{}) {
		t.Fatal("front-door Room name comparison does not match the unlocked door alone")
	}

	// Pay to unlock the back door: both names, both mana values summed.
	h.Emit(events.Event{Kind: events.DoorUnlock, Obj: id, Amount: 2})
	if !o.DoorUnlocked(1) {
		t.Fatalf("precondition: back door did not unlock: face0=%v face1=%v", o.DoorUnlocked(0), o.DoorUnlocked(1))
	}
	if names, _ := roomNames(o); !slices.Equal(names, []string{"Front Door", "Back Door"}) {
		t.Fatalf("both-door roomNames = %v, want both doors", names)
	}
	if got := objectManaValue(h.g, o); got != 5 {
		t.Fatalf("both-door objectManaValue = %d, want 5", got)
	}
	if !sharesName(o, "Front Door", SpecContext{}) || !sharesName(o, "Back Door", SpecContext{}) {
		t.Fatal("both-door Room does not share either door's name")
	}

	// Lock the cast door (Keys to the House): the other door alone remains.
	h.Emit(events.Event{Kind: events.DoorLock, Obj: id, Amount: 1})
	if o.DoorUnlocked(0) || !o.DoorUnlocked(1) {
		t.Fatalf("precondition: cast-door lock left face0=%v face1=%v", o.DoorUnlocked(0), o.DoorUnlocked(1))
	}
	if names, _ := roomNames(o); !slices.Equal(names, []string{"Back Door"}) {
		t.Fatalf("re-locked roomNames = %v, want [Back Door]", names)
	}
	if got := objectManaValue(h.g, o); got != 3 {
		t.Fatalf("re-locked objectManaValue = %d, want 3", got)
	}

	// Off the battlefield the rule does not function: the card is an
	// ordinary split card with both printed names and the combined mana
	// value of its halves (CR 709.4, 709.4b: 2+3).
	o.Zone = state.ZHand
	if _, room := roomNames(o); room {
		t.Fatal("a hand Room took the battlefield-only CR 709.5 rule")
	}
	if got := objectManaValue(h.g, o); got != 5 {
		t.Fatalf("hand objectManaValue = %d, want the combined halves 5 (CR 709.4b)", got)
	}
	if got := nameCharacteristics(o); !slices.Equal(got, []string{"Front Door", "Back Door"}) {
		t.Fatalf("hand nameCharacteristics = %v, want both printed halves", got)
	}
}
