package rules

// mayplay_case_locked_hothouse_test.go pins that a solved Case of the Locked
// Hothouse registers its printed MayPlay$ True ContinuousEffect (the printed
// whitelist used to refuse the IsPresent$/MayLookAt$ keys on the line).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// hothouseEngine puts the corpus Case on seat 0's battlefield (solved or not)
// with a top library card of the named corpus card.
func hothouseEngine(t *testing.T, solved bool, top string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := mayPlayBase(t)
	c := e.G.AddObject(corpusAlternativeCard(t, "Case of the Locked Hothouse"), 0)
	c.Zone = state.ZLibrary
	lib := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	e.G.SetZone(state.ZLibrary, 0, append(lib, c.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: c.ID, From: state.ZLibrary, To: state.ZBattlefield})
	if solved {
		e.emit(events.Event{Kind: events.AlterAttribute, Obj: c.ID, Text: "Solved", Amount: 1})
	}
	topObj := e.G.AddObject(corpusAlternativeCard(t, top), 0)
	topObj.Zone = state.ZLibrary
	e.G.SetZone(state.ZLibrary, 0, append([]state.ObjID{topObj.ID}, e.G.Zone(state.ZLibrary, 0)...))
	e.staticEpoch = -1
	e.activeEpoch = -1
	if e.G.Obj(c.ID).Zone != state.ZBattlefield || e.G.Zone(state.ZLibrary, 0)[0] != topObj.ID {
		t.Fatal("precondition: Case on the battlefield and the probe card on top of the library")
	}
	return e, c.ID, topObj.ID
}

func printedMayPlayCEs(e *Engine, src state.ObjID) int {
	n := 0
	for _, ce := range e.active() {
		if ce.Source == src && ce.MayPlay && !ce.FromEffect {
			n++
		}
	}
	return n
}

func hasSpellOffer(e *Engine, id state.ObjID) bool {
	for _, o := range e.mayPlaySpellIds(0) {
		if o.id == id {
			return true
		}
	}
	return false
}

func TestCaseOfTheLockedHothouseRegistersItsPrintedPlayGrant(t *testing.T) {
	e, caseID, top := hothouseEngine(t, true, "Forest")
	if got := printedMayPlayCEs(e, caseID); got != 1 {
		t.Fatalf("solved Case registered %d printed may-play CEs, want 1", got)
	}
	if !e.MayLookAtLibraryTop(0) {
		t.Fatal("solved Case does not let seat 0 look at the library top")
	}
	found := false
	for _, id := range e.mayPlayLandIds(0) {
		found = found || id == top
	}
	if !found {
		t.Fatalf("top Forest not playable: %v", e.mayPlayLandIds(0))
	}
	for _, tc := range []struct {
		name string
		want bool
	}{{"Grizzly Bears", true}, {"Rancor", true}, {"Doom Blade", false}} {
		e, _, top := hothouseEngine(t, true, tc.name)
		if got := hasSpellOffer(e, top); got != tc.want {
			t.Errorf("%s on top: cast offered = %v, want %v", tc.name, got, tc.want)
		}
	}

	e, caseID, top = hothouseEngine(t, false, "Forest")
	if got := printedMayPlayCEs(e, caseID); got != 0 {
		t.Fatalf("unsolved Case registered %d printed may-play CEs, want 0", got)
	}
	if e.MayLookAtLibraryTop(0) {
		t.Fatal("unsolved Case grants the library look")
	}
	for _, id := range e.mayPlayLandIds(0) {
		if id == top {
			t.Fatal("unsolved Case offers the top land")
		}
	}
}
