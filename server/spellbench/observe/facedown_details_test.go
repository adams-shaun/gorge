package observe_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

func chosenValue(entries []map[string]string, kind string) string {
	for _, e := range entries {
		if e["kind"] == kind {
			return e["value"]
		}
	}
	return ""
}

// TestFaceDownPermanentHidesChosenClassAndDoors pins that the permanent
// details this ticket added never leak a face-down permanent's hidden identity
// to an opponent: class_level stays null, chosen stays empty and the Room door
// statuses are omitted, while the public morph-like statuses still ride along.
// The controller, who may look, still sees all of them.
func TestFaceDownPermanentHidesChosenClassAndDoors(t *testing.T) {
	reg := testcorpus.Registry(t)
	deck := catalog.Deck{CatalogID: "FaceDownDetails", Rows: []wire.DeckRow{
		{Name: "Wizard Class", Count: 20}, {Name: "Unholy Annex // Ritual Chamber", Count: 20}}}
	cardsInDeck, err := catalog.Resolve(reg, deck)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 32)
	secret[0] = 12
	g, err := gamecfg.New(reg, secrets.NewGame(secret), [2][]*cards.Card{cardsInDeck, cardsInDeck},
		gamecfg.Rules{Mulligan: "none", StartingSeat: 0})
	if err != nil {
		t.Fatal(err)
	}
	var classID, roomID state.ObjID
	for i := range g.E.G.Objs {
		o := &g.E.G.Objs[i]
		if o.Owner != 0 || o.Face() == nil || o.Zone == state.ZBattlefield {
			continue
		}
		switch o.Face().Name {
		case "Wizard Class":
			if classID == 0 {
				classID = o.ID
			}
		case "Unholy Annex":
			if roomID == 0 {
				roomID = o.ID
			}
		}
	}
	if classID == 0 || roomID == 0 {
		t.Fatalf("deck did not contain both cards: class %d, room %d", classID, roomID)
	}
	move := func(id state.ObjID, marker string) {
		o := g.E.G.Obj(id)
		events.Emit(g.E.G, g.E.L, events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone,
			To: state.ZBattlefield, Counter: marker})
	}
	move(classID, events.FaceDownEntryCounter)
	move(roomID, events.CloakEntryCounter)
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.Choose, Obj: classID, Counter: "name", Text: "Goblin"})
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.DoorUnlock, Obj: roomID, Amount: 1})

	for _, c := range []struct {
		id   state.ObjID
		name string
	}{{classID, "Wizard Class"}, {roomID, "Unholy Annex"}} {
		o := g.E.G.Obj(c.id)
		if o.Zone != state.ZBattlefield || !o.FaceDown || o.Controller != 0 {
			t.Fatalf("precondition: %s zone %v, face down %v, controller %d", c.name, o.Zone, o.FaceDown, o.Controller)
		}
	}
	if o := g.E.G.Obj(classID); !strings.Contains(strings.Join(o.Face().Types, " "), "Class") ||
		o.ClassLevel() != 1 || o.ChosenName != "Goblin" {
		t.Fatalf("precondition: class types %v, level %d, chosen %q", o.Face().Types, o.ClassLevel(), o.ChosenName)
	}
	if o := g.E.G.Obj(roomID); !o.DoorUnlocked(0) || !o.Cloaked {
		t.Fatalf("precondition: room door0 %v, cloaked %v", o.DoorUnlocked(0), o.Cloaked)
	}

	tr := identity.New(g.E, g.Secret)
	if err := tr.Sync(g.E); err != nil {
		t.Fatal(err)
	}
	p := &observe.Projector{E: g.E, IDs: tr}

	// The controller may look at its own face-down permanent, so the details
	// are real and a test that always nil'd them would fail this half.
	ownClass, err := p.Record(0, classID)
	if err != nil {
		t.Fatal(err)
	}
	if ownClass.Permanent.ClassLevel == nil || *ownClass.Permanent.ClassLevel != 1 {
		t.Fatalf("controller class_level %v, want 1", ownClass.Permanent.ClassLevel)
	}
	if got := chosenValue(ownClass.Permanent.Chosen, "card_name"); got != "Goblin" {
		t.Fatalf("controller chosen %v, want card_name Goblin", ownClass.Permanent.Chosen)
	}
	ownRoom, err := p.Record(0, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ownRoom.Permanent.Statuses, "left_door") {
		t.Fatalf("controller statuses %v, want left_door", ownRoom.Permanent.Statuses)
	}

	// The opponent sees a face-down permanent: nothing above may leak.
	oppClass, err := p.Record(1, classID)
	if err != nil {
		t.Fatal(err)
	}
	if oppClass.CardName != nil {
		t.Fatalf("precondition: opponent sees face-down name %q", *oppClass.CardName)
	}
	if oppClass.Permanent.ClassLevel != nil {
		t.Fatalf("opponent class_level %v leaks the hidden Class type", *oppClass.Permanent.ClassLevel)
	}
	if len(oppClass.Permanent.Chosen) != 0 {
		t.Fatalf("opponent chosen %v leaks the hidden choice", oppClass.Permanent.Chosen)
	}
	oppRoom, err := p.Record(1, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(oppRoom.Permanent.Statuses, "left_door") || slices.Contains(oppRoom.Permanent.Statuses, "right_door") {
		t.Fatalf("opponent statuses %v leak the hidden Room doors", oppRoom.Permanent.Statuses)
	}
	if !slices.Contains(oppRoom.Permanent.Statuses, "cloaked") {
		t.Fatalf("opponent statuses %v dropped the public cloaked status", oppRoom.Permanent.Statuses)
	}
}
