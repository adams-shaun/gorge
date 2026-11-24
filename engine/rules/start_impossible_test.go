package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const startInstant = "Name:Start Instant\nManaCost:W\nTypes:Instant\nA:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ x\nOracle:x\n"

// startLibraryConsistent fails when an object of seat p sits in the library
// zone but outside the library list, or the reverse.
func startLibraryConsistent(t *testing.T, e *Engine, p state.PlayerID) {
	t.Helper()
	list := e.G.Zone(state.ZLibrary, p)
	for _, id := range list {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZLibrary {
			t.Fatalf("library list holds %d, which is not in the library zone", id)
		}
	}
	n := 0
	for id := state.ObjID(1); id <= state.ObjID(80); id++ {
		if o := e.G.Obj(id); o != nil && o.Owner == p && o.Zone == state.ZLibrary {
			n++
			if !slices.Contains(list, id) {
				t.Fatalf("object %d is in the library zone but not in the library list (%d listed)", id, len(list))
			}
		}
	}
	if n != len(list) {
		t.Fatalf("library zone holds %d objects, list holds %d", n, len(list))
	}
}

// CR 304.4 / 307.4: an instant or sorcery is never a permanent, so a start
// listing one on the battlefield is refused, and the setup leaves it out.
func TestStartRefusesNonPermanentOnBattlefield(t *testing.T) {
	start := &StartSpec{Seats: []StartSeat{{Battlefield: []int{1, 2}}}}
	special := [2]map[int]string{{1: startInstant}}
	cfg := startConfig(t, 7, start, special)
	if !cfg.Decks[0][1].Faces[0].IsInstant() {
		t.Fatalf("precondition: deck card 1 is not an instant")
	}
	err := start.Validate(cfg.Decks)
	if err == nil || !errors.Is(err, ErrStartNotPermanent) || !strings.Contains(err.Error(), "not a permanent") {
		t.Fatalf("Validate = %v, want a not a permanent error", err)
	}
	if err := (&StartSpec{Seats: []StartSeat{{Battlefield: []int{2}}}}).Validate(cfg.Decks); err != nil {
		t.Fatalf("Validate refused a creature: %v", err)
	}
	e := New(cfg)
	if f := zoneIdx(e, state.ZBattlefield, 0); slices.Contains(f, 1) || !slices.Contains(f, 2) {
		t.Fatalf("battlefield = %v, want the creature 2 and not the instant 1", f)
	}
	if !slices.Contains(zoneIdx(e, state.ZLibrary, 0), 1) {
		t.Fatalf("the refused instant is not in the library list")
	}
	startLibraryConsistent(t, e, 0)
}

// CR 303.4g: an Aura listed on the battlefield with no bearer does not
// enter; it stays in the library, and so in the library order.
func TestStartUnattachedAuraKeepsLibraryOrder(t *testing.T) {
	start := &StartSpec{Seats: []StartSeat{{Battlefield: []int{1, 2}}}}
	cfg := startConfig(t, 7, start, [2]map[int]string{{1: startAura}})
	e := New(cfg)
	aura := e.G.Obj(startObjOf(e, 0, 1))
	if aura == nil || aura.Zone != state.ZLibrary {
		t.Fatalf("precondition: the unattached Aura is not left in the library (zone %v)", aura.Zone)
	}
	if aura.Tapped {
		t.Fatalf("the Aura left in the library was tapped")
	}
	if !slices.Contains(zoneIdx(e, state.ZLibrary, 0), 1) {
		t.Fatalf("the Aura is in the library zone but not the library list")
	}
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != 40-1 {
		t.Fatalf("library count %d, want 39 (40 cards, one creature placed)", got)
	}
	startLibraryConsistent(t, e, 0)
}

// startObjOf is the object for seat p's deck index idx.
func startObjOf(e *Engine, p, idx int) state.ObjID {
	for id := state.ObjID(1); id <= state.ObjID(80); id++ {
		if o := e.G.Obj(id); o != nil && int(o.Owner) == p && deckIndex(e, id) == idx {
			return id
		}
	}
	return 0
}
