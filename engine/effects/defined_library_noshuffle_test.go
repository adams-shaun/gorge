package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A Defined$ move from the library back into it places the card and leaves the
// rest of the library in its order: Forge's `defined` flag clears
// shuffleMandatory, so only an explicit Shuffle$ True shuffles (Hurkyl, Master
// Wizard and Kenessos, Priest of Thassa put a remembered card on the bottom with
// no Shuffle$ parameter).
func TestDefinedLibraryToLibraryShufflesOnlyOnShuffleTrue(t *testing.T) {
	for _, tc := range []struct {
		line     string
		shuffles int
	}{
		{"DB$ ChangeZone | Defined$ Remembered | Origin$ Library | Destination$ Library | LibraryPosition$ -1", 0},
		{"DB$ ChangeZone | Defined$ Remembered | Origin$ Library | Destination$ Library | LibraryPosition$ -1 | Shuffle$ True", 1},
	} {
		h := newHost(t, 2)
		source := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Sorcery\nOracle:x\n"), 0)
		top := h.g.AddObject(mkCard(t, "Name:Top\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
		middle := h.g.AddObject(mkCard(t, "Name:Middle\nTypes:Artifact\nOracle:x\n"), 0)
		fetched := h.g.AddObject(mkCard(t, "Name:Fetched\nTypes:Artifact\nOracle:x\n"), 0)
		h.g.SetZone(state.ZLibrary, 0, []state.ObjID{fetched.ID, top.ID, middle.ID})
		for _, id := range []state.ObjID{fetched.ID, top.ID, middle.ID} {
			h.g.Obj(id).Zone = state.ZLibrary
		}

		Resolve(h, &Ctx{Controller: 0, Source: source.ID, Remembered: []state.Target{{Obj: fetched.ID}}}, sa(t, tc.line))
		shuffles := 0
		for _, e := range h.log {
			if e.Kind == events.Shuffle {
				shuffles++
			}
		}
		if shuffles != tc.shuffles {
			t.Fatalf("%s: %d Shuffle events, want %d: %+v", tc.line, shuffles, tc.shuffles, h.log)
		}
		lib := h.g.Zone(state.ZLibrary, 0)
		if len(lib) != 3 || lib[len(lib)-1] != fetched.ID {
			t.Fatalf("%s: library %v, want the fetched card %d on the bottom", tc.line, lib, fetched.ID)
		}
		if tc.shuffles == 0 && (lib[0] != top.ID || lib[1] != middle.ID) {
			t.Fatalf("%s: library %v, want the rest in order [%d %d]", tc.line, lib, top.ID, middle.ID)
		}
	}
}
