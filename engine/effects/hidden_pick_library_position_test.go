package effects

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// hpLibraryOrders counts the LibraryOrder events and fails on any Note that
// names LibraryPosition$ (the placement's loud fail-closed shape).
func hpLibraryOrders(t *testing.T, h *fakeHost) int {
	t.Helper()
	n := 0
	for _, e := range h.log {
		switch {
		case e.Kind == events.LibraryOrder:
			n++
		case e.Kind == events.Note && strings.Contains(e.Text, "LibraryPosition$"):
			t.Fatalf("unexpected LibraryPosition$ Note %+v", e)
		}
	}
	return n
}

// TestHiddenPickLibraryPositionTopLluwen drives the real corpus Lluwen,
// Imperfect Naturalist enters trigger (Mill 4 RememberMilled, then the hidden
// Graveyard,Exile pick with LibraryPosition$ 0, then cleanup): the picked
// creature card must land on TOP of the library. Before the fix the hidden
// pick never placed what it moved, so the MoveZone bottom append stood.
func TestHiddenPickLibraryPositionTopLluwen(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Lluwen, Imperfect Naturalist")
	if !ok {
		t.Fatal("corpus has no Lluwen, Imperfect Naturalist")
	}
	var head *cards.SA
	for _, tr := range card.Faces[0].Triggers {
		if tr.Effect != nil && tr.Effect.API == "Mill" {
			head = tr.Effect
			break
		}
	}
	if head == nil {
		t.Fatal("Lluwen has no Mill trigger effect in the corpus")
	}
	var pick *cards.SA
	for s := head; s != nil; s = s.Sub {
		if s.API == "ChangeZone" {
			pick = s
		}
	}
	if pick == nil || pick.Params["Hidden"] != "True" || pick.Params["Destination"] != "Library" ||
		pick.Params["LibraryPosition"] != "0" {
		t.Fatalf("compiled pick = %+v, want a Hidden$ ChangeZone to Library with LibraryPosition$ 0", pick)
	}

	h := newHost(t, 2)
	lluwen := lzAdd(t, h, 0, state.ZBattlefield, card)
	bear := lzCreature(t, h, 0, state.ZLibrary, "Bear")
	fills := lzFills(t, h, 5)
	if lib := lzLibraryOrderIDs(h, 0); len(lib) != 6 || lib[0] != bear {
		t.Fatalf("precondition: library = %v, want Bear on top of 5 fillers", lib)
	}

	Resolve(h, &Ctx{Controller: 0, Source: lluwen}, head)

	if z := h.g.Obj(bear).Zone; z != state.ZLibrary {
		t.Fatalf("Bear zone = %v, want library (the pick moved nothing)", z)
	}
	want := []state.ObjID{bear, fills[3], fills[4]}
	if lib := lzLibraryOrderIDs(h, 0); !slices.Equal(lib, want) {
		t.Fatalf("library = %v, want Bear on TOP: %v", lib, want)
	}
	if n := hpLibraryOrders(t, h); n != 1 {
		t.Fatalf("LibraryOrder events = %d, want exactly 1", n)
	}
}

// hpGraveyardPick resolves a hidden graveyard pick to the library at pos
// (absent when empty) and returns the host, the moved card and the fillers.
func hpGraveyardPick(t *testing.T, pos string) (*fakeHost, state.ObjID, []state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	fills := lzFills(t, h, 2)
	gy := lzCreature(t, h, 0, state.ZGraveyard, "Returned")
	if z := h.g.Obj(gy).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: zone = %v, want graveyard", z)
	}
	line := "DB$ ChangeZone | Origin$ Graveyard | Destination$ Library | Hidden$ True | ChangeType$ Creature.YouOwn | Mandatory$ True"
	if pos != "" {
		line += " | LibraryPosition$ " + pos
	}
	Resolve(h, &Ctx{Controller: 0, Source: gy}, sa(t, line))
	if z := h.g.Obj(gy).Zone; z != state.ZLibrary {
		t.Fatalf("zone after = %v, want library (the pick moved nothing)", z)
	}
	return h, gy, fills
}

// TestHiddenPickLibraryPositionAbsentIsTop: an absent LibraryPosition$ is
// Forge's TOP default on the hidden pick, as on the object-target path.
func TestHiddenPickLibraryPositionAbsentIsTop(t *testing.T) {
	h, gy, fills := hpGraveyardPick(t, "")
	want := append([]state.ObjID{gy}, fills...)
	if lib := lzLibraryOrderIDs(h, 0); !slices.Equal(lib, want) {
		t.Fatalf("library = %v, want the pick on TOP: %v", lib, want)
	}
	if n := hpLibraryOrders(t, h); n != 1 {
		t.Fatalf("LibraryOrder events = %d, want exactly 1", n)
	}
}

// TestHiddenPickLibraryPositionBottom: LibraryPosition$ -1 keeps the card at
// the bottom, now through an explicit placement.
func TestHiddenPickLibraryPositionBottom(t *testing.T) {
	h, gy, fills := hpGraveyardPick(t, "-1")
	want := append(slices.Clone(fills), gy)
	if lib := lzLibraryOrderIDs(h, 0); !slices.Equal(lib, want) {
		t.Fatalf("library = %v, want the pick at the BOTTOM: %v", lib, want)
	}
	if n := hpLibraryOrders(t, h); n != 1 {
		t.Fatalf("LibraryOrder events = %d, want exactly 1", n)
	}
}

// TestHiddenPickLibraryPositionOwnersLibrary: a battlefield pick (Whirlpool
// Whelm, Nulltread Gargantuan's shape) owned by seat 1 and controlled by seat
// 0 goes on top of its OWNER's library.
func TestHiddenPickLibraryPositionOwnersLibrary(t *testing.T) {
	h := newHost(t, 2)
	theirs := []state.ObjID{lzFiller(t, h, 1, "G0"), lzFiller(t, h, 1, "G1")}
	mine := lzFills(t, h, 2)
	stolen := lzCreature(t, h, 1, state.ZBattlefield, "Stolen")
	h.g.Obj(stolen).Controller = 0
	if o := h.g.Obj(stolen); o.Zone != state.ZBattlefield || o.Owner != 1 || o.Controller != 0 {
		t.Fatalf("precondition: object = zone %v owner %v controller %v, want battlefield/1/0", o.Zone, o.Owner, o.Controller)
	}
	line := "DB$ ChangeZone | Origin$ Battlefield | Destination$ Library | Hidden$ True | ChangeType$ Creature | Mandatory$ True | LibraryPosition$ 0"
	Resolve(h, &Ctx{Controller: 0, Source: stolen}, sa(t, line))

	if z := h.g.Obj(stolen).Zone; z != state.ZLibrary {
		t.Fatalf("zone after = %v, want library (the pick moved nothing)", z)
	}
	want := append([]state.ObjID{stolen}, theirs...)
	if lib := lzLibraryOrderIDs(h, 1); !slices.Equal(lib, want) {
		t.Fatalf("seat 1 library = %v, want the pick on TOP of its owner's library: %v", lib, want)
	}
	if lib := lzLibraryOrderIDs(h, 0); !slices.Equal(lib, mine) {
		t.Fatalf("seat 0 library = %v, want untouched %v", lib, mine)
	}
	if n := hpLibraryOrders(t, h); n != 1 {
		t.Fatalf("LibraryOrder events = %d, want exactly 1", n)
	}
}
