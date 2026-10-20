package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The library search's shuffle is mandatory (CR 701.23 / 701.24; cr-g09).
// These replace the retired search_mayshuffle pins: every card is the real
// compiled corpus script loaded through the gitignored corpus registry -- no
// Forge script text is committed.

// TestBoggartHarbingerSearchShufflesThenPlacesOnTop pins the tail order on
// the LibraryPosition$ shape: Boggart Harbinger's search ("reveal it, then
// shuffle and put the card on top") shuffles the rest of the library and
// then puts the found Goblin on TOP, with no question asked.
func TestBoggartHarbingerSearchShufflesThenPlacesOnTop(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Boggart Harbinger", "Goblin Piker")
	bh := searchMoveByName(t, e, "Boggart Harbinger", state.ZHand)
	var goblin state.ObjID
	rest := make([]state.ObjID, 0, 16)
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		if o.Face().Name == "Goblin Piker" && goblin == 0 {
			goblin = id
			continue
		}
		rest = append(rest, id)
	}
	if goblin == 0 {
		t.Fatal("the seeded Goblin Piker is not in the library")
	}
	// Plant the Goblin on top (a logged LibraryOrder, the same event the
	// engine's placements emit, so the replay still verifies).
	order := append([]state.ObjID{goblin}, rest...)
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order, Secret: true})

	e.emit(events.Event{Kind: events.MoveZone, Obj: bh, From: state.ZHand, To: state.ZBattlefield})
	e.Advance()
	d := drainUntilAsk(t, e, 30)
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("Boggart's optional-trigger ask missing: %+v", d)
	}
	yesIdx := -1
	for _, o := range d.Options {
		if o.Kind == "yes" {
			yesIdx = o.Index
		}
	}
	submitChoices(t, e, yesIdx)
	d = drainUntilAsk(t, e, 30)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search" || len(d.Options) != 1 || d.Options[0].Obj != goblin {
		t.Fatalf("search pick = %+v, want the one Goblin Piker (%d)", d, goblin)
	}
	start := len(e.L.Events)
	submitChoices(t, e, d.Options[0].Index)
	if p := e.Pending(); p != nil && p.ResumeKind == "search_mayshuffle" {
		t.Fatalf("the search posed %+v; its shuffle is mandatory", p)
	}
	shuffles := 0
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			shuffles++
		}
	}
	if shuffles != 1 {
		t.Fatalf("shuffles after the search = %d, want 1", shuffles)
	}
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) == 0 || lib[0] != goblin {
		t.Fatalf("library top after the search = %v, want the Goblin Piker %d on top", lib, goblin)
	}
	if len(lib) != len(rest)+1 {
		t.Fatalf("library size %d, want %d (the Goblin plus the rest)", len(lib), len(rest)+1)
	}
	replayCheck(t, e, cfg)
}

// TestMultiPlayerSearchShufflesBeforeNextSearch uses Veteran Explorer's real
// death trigger. Seat 0 accepts its search and finds a basic land; seat 0's
// library is shuffled at once (no confirm), and the next ask is seat 1's own
// search confirmation (CR 701.23, each player in turn).
func TestMultiPlayerSearchShufflesBeforeNextSearch(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Veteran Explorer")
	veteran := searchMoveByName(t, e, "Veteran Explorer", state.ZHand)
	e.pending = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: veteran, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: veteran, From: state.ZBattlefield, To: state.ZGraveyard})
	if o := e.G.Obj(veteran); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Veteran Explorer = %+v, want it in the graveyard", o)
	}
	e.Advance()
	confirm := passUntilNonPriority(t, e, 40)
	if confirm == nil || confirm.Kind != decision.KChoose || confirm.ResumeKind != "search_confirm" || confirm.Player != 0 {
		t.Fatalf("first library ask = %+v, want seat 0 search confirmation", confirm)
	}
	submitChoices(t, e, confirm.Options[0].Index)
	search := passUntilNonPriority(t, e, 40)
	if search == nil || search.Kind != decision.KChoose || search.ResumeKind != "search" || search.Player != 0 {
		t.Fatalf("first library search = %+v, want seat 0 search", search)
	}
	if len(search.Options) == 0 {
		t.Fatal("precondition: seat 0's library search has no basic-land option")
	}
	start := len(e.L.Events)
	submitChoices(t, e, search.Options[0].Index)
	shuffles := 0
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			shuffles++
		}
	}
	if shuffles != 1 {
		t.Fatalf("seat-0 shuffles after its search = %d, want 1", shuffles)
	}
	next := e.Pending()
	if next == nil || next.Kind != decision.KChoose || next.ResumeKind != "search_confirm" || next.Player != 1 {
		t.Fatalf("pending after seat 0's search = %+v, want seat 1's search confirmation", next)
	}
}
