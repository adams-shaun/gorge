package effects

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDigMultipleSourceZoneNonLibraryFailsClosed: gorge digs the library only,
// so a SourceZone$ other than Library notes the gap and moves nothing, while
// SourceZone$ Library behaves as before.
func TestDigMultipleSourceZoneNonLibraryFailsClosed(t *testing.T) {
	const line = "SP$ DigMultiple | DigNum$ 4 | Reveal$ True | ChangeValid$ Creature,Land | Optional$ True | DestinationZone2$ Graveyard"
	h, ids := digMultipleBoard(t)
	h.choices = []int{1, 2}
	Resolve(h, &Ctx{Controller: 0}, sa(t, line+" | SourceZone$ Graveyard"))
	var note string
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "DigMultiple") {
			note = ev.Text
		}
		if ev.Kind == events.MoveZone {
			t.Fatalf("a non-Library SourceZone$ moved a card: %+v", ev)
		}
	}
	if note != "DigMultiple SourceZone$ Graveyard unimplemented" {
		t.Fatalf("note = %q", note)
	}
	if len(h.asked) != 0 || !reflect.DeepEqual(h.g.Zone(state.ZLibrary, 0), ids) {
		t.Fatalf("asked=%d library=%v, want untouched", len(h.asked), h.g.Zone(state.ZLibrary, 0))
	}

	// Precondition: the identical line with SourceZone$ Library does dig.
	h2, _ := digMultipleBoard(t)
	h2.choices = []int{1, 2}
	Resolve(h2, &Ctx{Controller: 0}, sa(t, line+" | SourceZone$ Library"))
	if len(h2.asked) == 0 || len(h2.g.Zone(state.ZHand, 0)) != 2 {
		t.Fatalf("SourceZone$ Library did not dig: asked=%d hand=%v", len(h2.asked), h2.g.Zone(state.ZHand, 0))
	}
}
