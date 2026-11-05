package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Room cast as its other half and later unlocked has a door whose trigger
// resolves a DB$ Effect naming a StaticAbilities$ body. gorge must look that
// body up on the trigger door's own SVar table, not the shown half's
// (CR 709.5, 113.7a; rules/rooms.go queueUnlockTriggers binds
// TriggerSVars: face.SVars): the front half here is the trigger door and
// only it defines STFly, while the cast half is the back one. If the lookup
// falls back to the shown face the Effect registers nothing and the
// creature never gains Flying.
func TestRoomOtherDoorEffectStaticReadsItsTable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	room := card(t, "Name:Front Door\nManaCost:0\nTypes:Enchantment Room\nOracle:x\n"+
		"T:Mode$ UnlockDoor | ValidPlayer$ You | ValidCard$ Card.Self | ThisDoor$ True | Execute$ TrigGrant | TriggerDescription$ x\n"+
		"SVar:TrigGrant:DB$ Effect | StaticAbilities$ STFly | Duration$ EndOfTurn\n"+
		"SVar:STFly:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ Flying | Description$ x\n"+
		"ALTERNATE\nName:Back Door\nManaCost:0\nTypes:Enchantment Room\nOracle:y\nAlternateMode:Split\n")
	bear := card(t, bearSrc)
	e := corpusEngine(t, reg, []*cards.Card{room, bear, bear, bear}, nil)
	bearID := moveByName(t, e, 0, "Bear", state.ZBattlefield)
	id := moveByName(t, e, 0, "Front Door", state.ZBattlefield)

	// The Room is live as its back half: the front door is the locked one,
	// and only the front half defines the Effect's named static.
	e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: 1})
	designateRoomCastFace(e, id)
	o := e.G.Obj(id)
	if o == nil || o.FaceIdx != 1 || !o.DoorUnlocked(1) || o.DoorUnlocked(0) {
		t.Fatalf("precondition: Room is not live as its back half with only the front door locked: %+v", o)
	}
	front, back := o.Card.Faces[0], o.Card.Faces[1]
	if front.SVars["STFly"] == "" || back.SVars["STFly"] != "" {
		t.Fatalf("precondition: STFly must live only on the unlocked front door (front %q, back %q)", front.SVars["STFly"], back.SVars["STFly"])
	}
	if e.HasKeyword(bearID, "Flying") {
		t.Fatal("precondition: the Bear already has Flying before the unlock trigger")
	}

	e.emit(events.Event{Kind: events.DoorUnlock, Obj: id})
	if o = e.G.Obj(id); !o.DoorUnlocked(0) || !o.DoorUnlocked(1) {
		t.Fatalf("precondition: the unlock did not unlock both doors: face0=%v face1=%v", o.DoorUnlocked(0), o.DoorUnlocked(1))
	}
	e.priorityRound()
	if len(e.G.Stack) != 1 {
		t.Fatalf("the unlocked front door's trigger is not on the stack: %d items", len(e.G.Stack))
	}
	resolveStack(t, e)
	if !e.HasKeyword(bearID, "Flying") {
		t.Fatal("the Effect's STFly static did not apply: gorge resolved the named body on the shown face's table")
	}
}
