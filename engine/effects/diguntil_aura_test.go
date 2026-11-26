package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestDigUntilNonLiteralAmountWithheldStillResolves pins the rider that
// remains withheld: a non-literal Amount$ with no SVar in the resolving
// context still emits its one loud Note. DigZone$ PlanarDeck is the planar
// arm now (the planar dig): in this fixture's game with no planar deck the
// scan digs an empty deck, so nothing is found, the Aura stays in the
// library, and no DigZone$ Note is withheld.
func TestDigUntilNonLiteralAmountWithheldStillResolves(t *testing.T) {
	h, ids := digUntilFixture(t)
	ability := sa(t, "SP$ DigUntil | Valid$ Aura | Amount$ X | DigZone$ PlanarDeck | FoundDestination$ Hand | RevealedDestination$ Graveyard")

	// PRECONDITION: the matching Aura is in the scanned library.
	if h.g.Obj(ids[1]).Zone != state.ZLibrary {
		t.Fatalf("setup: matching Aura is not in the library: %s", h.g.Obj(ids[1]).Zone)
	}
	Resolve(h, &Ctx{Controller: 0}, ability)
	if h.g.Obj(ids[1]).Zone != state.ZLibrary {
		t.Fatalf("a dig of the seat's absent planar deck moved the Aura: zone = %s, want library", h.g.Obj(ids[1]).Zone)
	}
	want := []string{"Amount$ X"}
	var notes []string
	for _, ev := range h.log {
		if strings.HasPrefix(ev.Text, "DigUntil withholds ") {
			notes = append(notes, ev.Text)
		}
	}
	if len(notes) != len(want) {
		t.Fatalf("withheld Notes = %d, want one per unsupported parameter %v: %v", len(notes), want, notes)
	}
	for _, param := range want {
		found := false
		for _, note := range notes {
			if strings.Contains(note, "withholds "+param+";") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing withheld-param Note for %s: %v", param, notes)
		}
	}
}
