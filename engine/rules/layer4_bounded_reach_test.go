package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// layer4AuraScript is Witness Protection's layer-4 shape: an Aura whose
// static retypes only the enchanted creature (Creature.EnchantedBy), plus a
// Squire-shaped self static gated on counters.
const layer4AuraScript = "Name:Retype Aura\nManaCost:U\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
	"S:Mode$ Continuous | Affected$ Creature.EnchantedBy | AddType$ Creature & Citizen | RemoveCardTypes$ True | RemoveCreatureTypes$ True | Description$ x\nOracle:x\n"

const layer4SquireScript = "Name:Counter Squire\nManaCost:1 W\nTypes:Creature Cat Scout\nPT:1/1\n" +
	"S:Mode$ Continuous | Affected$ Card.Self+counters_GE3_P1P1 | AddType$ Knight | Description$ x\nOracle:x\n"

// TestLayer4BoundedReachMatchesFullWalk drives the derived-type table through
// static-derived layer-4 effects with a bounded reach (the enchanted creature,
// the source itself) while unrelated objects come and go. Every refresh runs
// under layer4PrecheckVerify, which rebuilds the table with the whole-board
// walk and panics on any difference; the test also pins the reach and the
// table contents at each step.
func TestLayer4BoundedReachMatchesFullWalk(t *testing.T) {
	aura, squire := card(t, layer4AuraScript), card(t, layer4SquireScript)
	deck := append(mountainDeck(t, 38), aura, squire)
	e := New(seatZeroStart(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}}))
	if !e.layer4InPool {
		t.Fatal("precondition: the retyping cards did not arm layer4InPool")
	}
	bear := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	other := onBoard(t, e, 1, "Name:Other\nManaCost:1 G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n")
	sq := onBoard(t, e, 0, layer4SquireScript)
	au := onBoard(t, e, 0, layer4AuraScript)
	e.Emit(events.Event{Kind: events.Attach, Obj: au, IDs: []state.ObjID{bear}})
	reach, anyLType, ok := e.layer4BoundedReach(nil)
	if !ok || !anyLType || !slices.Contains(reach, bear) || !slices.Contains(reach, sq) || slices.Contains(reach, other) {
		t.Fatalf("bounded reach = %v (any %v ok %v), want the enchanted Bear and the Squire only", reach, anyLType, ok)
	}
	if ty := e.Derived(bear).Types; !slices.Contains(ty, "Citizen") || slices.Contains(ty, "Bear") {
		t.Fatalf("enchanted Bear derives %v, want the Citizen retype", ty)
	}
	// Grow the Squire past its gate one counter at a time, minting and
	// moving unrelated objects in between: every refresh is verified.
	builds := e.typesIncrBuilds
	for i := 0; i < 3; i++ {
		e.Emit(events.Event{Kind: events.CounterChange, Obj: sq, Counter: "P1P1", Amount: 1})
		onBoard(t, e, 1, "Name:Filler\nManaCost:1\nTypes:Creature Rat\nPT:1/1\nOracle:x\n")
		e.Emit(events.Event{Kind: events.Tap, Obj: other})
		e.Emit(events.Event{Kind: events.Untap, Obj: other})
	}
	if e.typesIncrBuilds == builds {
		t.Fatal("no refresh took the incremental bounded-reach path")
	}
	if ty := e.Derived(sq).Types; !slices.Contains(ty, "Knight") {
		t.Fatalf("Squire with three counters derives %v, want Knight", ty)
	}
	found := false
	for _, ot := range e.EffectiveTypes() {
		if ot.ID == other {
			t.Fatalf("unreached object %d has a table entry %v", other, ot.Types)
		}
		found = found || ot.ID == bear
	}
	if !found {
		t.Fatalf("table %v carries no entry for the enchanted Bear", e.EffectiveTypes())
	}
	// Move the Aura onto the other creature: the reach follows it.
	e.Emit(events.Event{Kind: events.Attach, Obj: au, IDs: []state.ObjID{other}})
	if ty := e.Derived(other).Types; !slices.Contains(ty, "Citizen") {
		t.Fatalf("re-enchanted creature derives %v, want Citizen", ty)
	}
	if ty := e.Derived(bear).Types; !slices.Contains(ty, "Bear") {
		t.Fatalf("released Bear derives %v, want its printed Bear back", ty)
	}
	e.Emit(events.Event{Kind: events.MoveZone, Obj: au, From: state.ZBattlefield, To: state.ZGraveyard})
	if ty := e.Derived(other).Types; !slices.Contains(ty, "Elf") {
		t.Fatalf("creature derives %v after the Aura left, want its printed Elf", ty)
	}
}

func TestAffectsReach(t *testing.T) {
	for _, c := range []struct {
		spec string
		want uint8
	}{
		{"Card.Self", reachSelf},
		{"Card.Self+counters_GE3_P1P1", reachSelf},
		{"Creature.EnchantedBy", reachAttached},
		{"Creature.EquippedBy+YouCtrl", reachAttached},
		{"Creature.Self,Creature.EnchantedBy", reachSelf | reachAttached},
		{"Creature.YouCtrl", 0},
		{"Creature.Self,Creature.YouCtrl", 0},
		{"Creature.!Self", 0},
		{"Creature.namedFoo+Self", 0},
		{"Creature.ControlledBy TriggeredTarget+Self", 0},
		{"Self", 0},
		{"", 0},
	} {
		if got := affectsReach(c.spec); got != c.want {
			t.Errorf("affectsReach(%q) = %d, want %d", c.spec, got, c.want)
		}
	}
}

// TestRenameBoundedReachMatchesFullWalk is the rename table's twin of the
// bounded layer-4 walk: Witness Protection's SetName$ on the enchanted
// creature only. Every refresh runs under layerInertVerify, which rebuilds the
// table with the whole-battlefield walk and panics on a difference.
func TestRenameBoundedReachMatchesFullWalk(t *testing.T) {
	const witness = "Name:Rename Aura\nManaCost:U\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"S:Mode$ Continuous | Affected$ Creature.EnchantedBy | SetName$ Legitimate Businessperson | Description$ x\nOracle:x\n"
	aura := card(t, witness)
	deck := append(mountainDeck(t, 39), aura)
	e := New(seatZeroStart(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}}))
	if !e.setNameInPool {
		t.Fatal("precondition: the renaming Aura did not arm setNameInPool")
	}
	bear := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	other := onBoard(t, e, 1, "Name:Other\nManaCost:1 G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n")
	au := onBoard(t, e, 0, witness)
	e.Emit(events.Event{Kind: events.Attach, Obj: au, IDs: []state.ObjID{bear}})
	if reach, ok := e.setNameBoundedReach(nil); !ok || !slices.Equal(reach, []state.ObjID{bear}) {
		t.Fatalf("rename reach = %v (ok %v), want the enchanted Bear", reach, ok)
	}
	if got := e.EffectiveNames(); len(got) != 1 || got[0].ID != bear || got[0].Name != "Legitimate Businessperson" {
		t.Fatalf("rename table %v, want the Bear renamed", got)
	}
	e.Emit(events.Event{Kind: events.Attach, Obj: au, IDs: []state.ObjID{other}})
	e.Emit(events.Event{Kind: events.Tap, Obj: bear})
	if got := e.EffectiveNames(); len(got) != 1 || got[0].ID != other {
		t.Fatalf("rename table %v after the move, want only the newly enchanted creature", got)
	}
}

// A manifested Aura is a face-down creature with no abilities, so the Aura's
// own layer-4 static is not live while it is face down. The type table the
// incremental build keeps is then empty (no LType effect is in active()), and
// the verify-mode reference must be the gated full build, not the ungated walk,
// which lists the face-down permanent. Seating it must neither panic under
// layer4PrecheckVerify nor stop it being a creature.
func TestLayer4TypeTableVerifyAcceptsFaceDownAuraWithNoLiveEffect(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	aura := lookup(t, reg, "Unable to Scream")
	e := quietBaseWith(t, reg, []*cards.Card{aura})
	id := pullByName(t, e, 0, "Unable to Scream")
	from := e.G.Obj(id).Zone
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, Player: 0, From: from, To: state.ZBattlefield,
		Counter: events.ManifestEntryCounter, Secret: true})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || !o.FaceDown {
		t.Fatalf("precondition: the Aura is not a face-down permanent: %+v", o)
	}
	if !e.IsCreature(id) {
		t.Fatalf("a face-down permanent must be a creature, got types %v", e.derivedTypesOf(id))
	}
}
