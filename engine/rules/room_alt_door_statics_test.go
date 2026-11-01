package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// castRoomThenUnlock casts the door named cast and unlocks the other door, so
// the other door is live though it is not the face the permanent was cast as
// (CR 709.5e).
func castRoomThenUnlock(t *testing.T, e *Engine, card, cast, mana string) state.ObjID {
	t.Helper()
	moveByName(t, e, 0, card, state.ZHand)
	addMana(t, e, 0, mana)
	castNamed(t, e, cast)
	settleRoom(t, e)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if !isRoom(o) || o.Card.Faces[0].Name != card {
			continue
		}
		other := 1 - int(o.FaceIdx)
		addMana(t, e, 0, mana)
		d := e.Pending()
		for _, opt := range d.Options {
			if opt.Kind == "unlock" && opt.Obj == id {
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
					t.Fatalf("unlock: %v", err)
				}
				break
			}
		}
		settleRoom(t, e)
		if !o.DoorUnlocked(other) {
			t.Fatalf("door %d of %s did not unlock", other, card)
		}
		return id
	}
	t.Fatalf("%s is not on the battlefield", card)
	return 0
}

// settleRoom passes and answers until the stack and the trigger queue are empty.
// Every ask a door's trigger poses here may be answered with its minimum.
func settleRoom(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 200; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending")
		}
		if d.Kind == decision.KPriority && len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
			return
		}
		var choices []int
		for k := 0; k < d.Min && k < len(d.Options); k++ {
			choices = append(choices, d.Options[k].Index)
		}
		if d.Kind == decision.KPriority {
			for _, o := range d.Options {
				if o.Kind == "pass" {
					choices = []int{o.Index}
				}
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
			t.Fatalf("answer %v: %v", d.Kind, err)
		}
	}
	t.Fatal("the stack did not settle")
}

func roomEngine(t *testing.T, names ...string) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	extras := make([]*cards.Card, 0, len(names))
	for _, n := range names {
		extras = append(extras, lookup(t, reg, n))
	}
	return corpusEngine(t, reg, extras, nil)
}

// CR 709.5: an unlocked door's static functions whichever door was cast. The
// first door of Mirror Room is the cast one here, so Fractured Realm's doubling
// is the alternate door's.
func TestUnlockedAltDoorPanharmoniconFunctions(t *testing.T) {
	t.Parallel()
	e := roomEngine(t, "Mirror Room", "Spiked Corridor")
	castRoomThenUnlock(t, e, "Mirror Room", "Mirror Room", "UUUUUUU")
	moveByName(t, e, 0, "Spiked Corridor", state.ZHand)
	addMana(t, e, 0, "RRRR")
	castNamed(t, e, "Spiked Corridor")
	passUntilStackEmpty(t, e, 30)
	answerQuiet(t, e, 60)
	if got := countDevils(e); got != 6 {
		t.Fatalf("Fractured Realm on the alternate door left %d Devils, want 6", got)
	}
}

func TestLockedAltDoorStaticStaysInert(t *testing.T) {
	t.Parallel()
	e := roomEngine(t, "Mirror Room")
	moveByName(t, e, 0, "Mirror Room", state.ZHand)
	addMana(t, e, 0, "UUU")
	castNamed(t, e, "Mirror Room")
	passUntilStackEmpty(t, e, 30)
	answerQuiet(t, e, 60)
	if got := len(e.activeStatics("Panharmonicon")); got != 0 {
		t.Fatalf("locked Fractured Realm contributes %d statics, want 0", got)
	}
}

func TestUnlockedAltDoorUntapOtherPlayerFunctions(t *testing.T) {
	t.Parallel()
	e := roomEngine(t, "Dazzling Theater", "Grizzly Bears")
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	if e.untapOtherStaticsMatch(bear) {
		t.Fatal("precondition: the bear untaps with no Prop Room")
	}
	castRoomThenUnlock(t, e, "Dazzling Theater", "Dazzling Theater", "WWWW")
	if !e.untapOtherStaticsMatch(bear) {
		t.Fatal("Prop Room on the alternate door does not untap the bear")
	}
}

// Neither door is live on a Room that entered uncast, so Prop Room's static is
// not either.
func TestUncastRoomUntapOtherPlayerInert(t *testing.T) {
	t.Parallel()
	e := roomEngine(t, "Dazzling Theater", "Grizzly Bears")
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	moveByName(t, e, 0, "Dazzling Theater", state.ZBattlefield)
	if e.untapOtherStaticsMatch(bear) {
		t.Fatal("a locked Room untaps the bear")
	}
}

// Greenhouse is the first door: casting Rickety Gazebo leaves it the alternate.
func TestUnlockedAltDoorGrantsManaAbility(t *testing.T) {
	t.Parallel()
	e := roomEngine(t, "Greenhouse", "Forest")
	land := moveByName(t, e, 0, "Forest", state.ZBattlefield)
	e.G.Obj(land).SummonSick = false
	before := len(e.availableManaAbilities(0, land))
	castRoomThenUnlock(t, e, "Greenhouse", "Rickety Gazebo", "GGGG")
	if got := len(e.availableManaAbilities(0, land)); got != before+1 {
		t.Fatalf("mana abilities of the Forest: %d with Greenhouse unlocked, %d before", got, before)
	}
}

// A static on a Room's locked face grants nothing: an uncast Room has neither
// door unlocked (CR 709.5d).
func TestLockedDoorGrantsNoManaAbility(t *testing.T) {
	t.Parallel()
	e := roomEngine(t, "Greenhouse", "Forest")
	land := moveByName(t, e, 0, "Forest", state.ZBattlefield)
	before := len(e.availableManaAbilities(0, land))
	moveByName(t, e, 0, "Greenhouse", state.ZBattlefield)
	if got := len(e.availableManaAbilities(0, land)); got != before {
		t.Fatalf("mana abilities of the Forest: %d with a locked Greenhouse, %d before", got, before)
	}
}

// Walk-In Closet is the first door. Its permission plays a land from the
// graveyard whichever door was cast.
func TestUnlockedDoorGrantsMayPlay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, cast string }{
		{"cast door", "Walk-In Closet"},
		{"alternate door", "Forgotten Cellar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := roomEngine(t, "Walk-In Closet", "Forest")
			land := moveByName(t, e, 0, "Forest", state.ZGraveyard)
			if got := priorityOptionsOn(t, e, "play_land", land); got != 0 {
				t.Fatalf("precondition: %d play options for the graveyard Forest", got)
			}
			castRoomThenUnlock(t, e, "Walk-In Closet", tc.cast, "GGGGG")
			if got := priorityOptionsOn(t, e, "play_land", land); got != 1 {
				t.Fatalf("%d play options for the graveyard Forest with Walk-In Closet unlocked, want 1", got)
			}
		})
	}
}

// priorityOptionsOn counts the pending priority decision's options of kind on obj.
func priorityOptionsOn(t *testing.T, e *Engine, kind string, obj state.ObjID) int {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision: %+v", d)
	}
	n := 0
	for _, o := range d.Options {
		if o.Kind == kind && o.Obj == obj {
			n++
		}
	}
	return n
}
