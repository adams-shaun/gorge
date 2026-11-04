package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDiscoverOffersTheFoundCardItsTriggerCaptured pins Data, Exploring
// Humanity's discover on the card whose draw triggered it: the second card
// drawn this turn is Harmonize, captured as the triggering card, and it is put
// back on top before the trigger resolves (Brainstorm's shape). Back in the
// library it is a new object (CR 400.7), and discover exiles it as the found
// card, which its controller may cast without paying its mana cost (CR
// 701.57a). The Play over the discover's remembered card must not drop it as
// the trigger's capture.
func TestDiscoverOffersTheFoundCardItsTriggerCaptured(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoard(t, reg, "Data, Exploring Humanity")
	data := ids["Data, Exploring Humanity"]
	if z := e.G.Obj(data).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: Data in %s, want battlefield", z)
	}
	harm := e.G.AddObject(mustCorpusCard(t, reg, "Harmonize"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: harm.ID, From: harm.Zone, To: state.ZLibrary})
	first := state.ObjID(0)
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if id != harm.ID {
			first = id
			break
		}
	}
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: first, From: state.ZLibrary, To: state.ZHand})
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: harm.ID, From: state.ZLibrary, To: state.ZHand})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("precondition: %d pending triggers after the second draw, want Data's one", len(e.pendingTriggers))
	}
	moveToLibraryTop(t, e, harm.ID)
	lib := e.G.Zone(state.ZLibrary, 0)
	order := append([]state.ObjID{harm.ID}, lib[:len(lib)-1]...)
	if lib[len(lib)-1] != harm.ID {
		t.Fatalf("precondition: Harmonize did not return to the library's end: %v", lib)
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order})
	if lib := e.G.Zone(state.ZLibrary, 0); len(lib) == 0 || lib[0] != harm.ID {
		t.Fatalf("precondition: Harmonize is not on top of the library: %v", lib)
	}
	e.Advance()
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while Data's trigger was waiting")
		}
		if d.Kind == decision.KModes {
			for _, o := range d.Options {
				if o.Obj == harm.ID {
					if z := e.G.Obj(harm.ID).Zone; z != state.ZExile {
						t.Fatalf("Harmonize offered from %s, want exile", z)
					}
					hand := len(e.G.Zone(state.ZHand, 0))
					submitChoices(t, e, o.Index)
					passUntilStackEmpty(t, e, 20)
					if z := e.G.Obj(harm.ID).Zone; z != state.ZGraveyard {
						t.Fatalf("Harmonize cast from the discover is in %s, want graveyard", z)
					}
					if got := len(e.G.Zone(state.ZHand, 0)); got != hand+3 {
						t.Fatalf("hand %d after the free Harmonize, want %d", got, hand+3)
					}
					return
				}
			}
			t.Fatalf("discover's play offer lacks Harmonize: %+v", d.Options)
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision %q: %+v", d.Kind, d.Options)
		}
		if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
			break
		}
		passFirst(t, e)
	}
	t.Fatalf("Data's discover made no play offer; Harmonize is in %s", e.G.Obj(harm.ID).Zone)
}
