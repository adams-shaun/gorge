package host

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestLibraryOrderBodyCarriesNoIDsForItsOwner: the seat-scoped serving path
// (eventBodiesFor, behind the seat event stream and EventsSeat) ships a
// Secret LibraryOrder to the library's OWN seat with no ids on the wire
// (protocol.Event.IDs) and the same transcript line every other seat gets
// (CR 401.2; cr-g61).
func TestLibraryOrderBodyCarriesNoIDsForItsOwner(t *testing.T) {
	g, _, _ := shareFixture(t)
	order := events.Event{Seq: 5, Kind: events.LibraryOrder, Player: 0, IDs: []state.ObjID{3, 2, 1}, Secret: true}
	evs := []events.Event{order}
	own := eventBodiesFor(0, view.Seat, g, evs, false)
	other := eventBodiesFor(1, view.Seat, g, evs, false)
	if len(own) != 1 || len(other) != 1 {
		t.Fatalf("want one body per viewer, got %d and %d", len(own), len(other))
	}
	if own[0].Event.IDs != nil || own[0].Event.Kind != "library_order" || own[0].Event.Player != 0 || !own[0].Event.Secret {
		t.Fatalf("the owner's body = %+v, want the LibraryOrder's shape with no ids", own[0].Event)
	}
	if own[0].Line != other[0].Line || own[0].Line == "" {
		t.Fatalf("owner line %q, other seat line %q: want the same non-empty line", own[0].Line, other[0].Line)
	}
}
