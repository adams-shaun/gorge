package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCR401_5PlayWithTopCardRevealedShowsEveryPlayer: CR 401.5 names
// "play with the top card of their library revealed", and a revealed card
// is shown to every player (CR 701.20a). Courser of Kruphix's MayLookAt$
// Player is that wording (Forge resolves Player to every player): seat 1's
// view carries seat 0's top card, and so does seat 0's own.
func TestCR401_5PlayWithTopCardRevealedShowsEveryPlayer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Courser of Kruphix")}, nil)
	moveByName(t, e, 0, "Courser of Kruphix", state.ZBattlefield)
	top := e.G.Zone(state.ZLibrary, 0)[0]
	for _, viewer := range []state.PlayerID{0, 1} {
		v := view.Project(e.G, e, viewer, e.Pending())
		if lt := v.Players[0].LibraryTop; lt == nil || lt.ID != top {
			t.Fatalf("CR 401.5 seq %d: seat %d's view of seat 0's revealed top card = %+v, want card %d", len(e.L.Events), viewer, lt, top)
		}
	}
	if v := view.Project(e.G, e, 0, e.Pending()); v.Players[1].LibraryTop != nil {
		t.Fatalf("CR 401.5 seq %d: Courser reveals only its controller's library, but seat 1's top card is shown: %+v", len(e.L.Events), v.Players[1].LibraryTop)
	}
}

// TestCR401_5MayLookAtYouStaysPrivate is the guard: Vizier of the
// Menagerie's "You may look at the top card of your library any time"
// (MayLookAt$ You) is a private look, not a reveal: seat 0 sees its top
// card, seat 1 does not.
func TestCR401_5MayLookAtYouStaysPrivate(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Vizier of the Menagerie")}, nil)
	moveByName(t, e, 0, "Vizier of the Menagerie", state.ZBattlefield)
	top := e.G.Zone(state.ZLibrary, 0)[0]
	if lt := view.Project(e.G, e, 0, e.Pending()).Players[0].LibraryTop; lt == nil || lt.ID != top {
		t.Fatalf("CR 401.5 seq %d: Vizier's controller does not see the top card: %+v", len(e.L.Events), lt)
	}
	if lt := view.Project(e.G, e, 1, e.Pending()).Players[0].LibraryTop; lt != nil {
		t.Fatalf("CR 401.5 seq %d: Vizier's private look leaked to seat 1: %+v", len(e.L.Events), lt)
	}
}
