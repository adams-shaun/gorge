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

// TestCR401_5HandRevealGrantNamesNoLibraryCard: Telepathy ("Your opponents
// play with their hands revealed") and Enduring Renewal ("Play with your hand
// revealed") are MayLookAt$ Player over AffectedZone$ Hand. They name no
// library card, so neither the controller nor the opponent is shown a
// library top through them (CR 401.5 reveals only a library top the effect
// names).
func TestCR401_5HandRevealGrantNamesNoLibraryCard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		card   string
		seat   state.PlayerID // the seat whose view must stay empty
		target state.PlayerID // the library owner the Affected$ spec covers
	}{
		{"Telepathy", 0, 1},
		{"Enduring Renewal", 1, 0},
	} {
		e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, tc.card)}, nil)
		moveByName(t, e, 0, tc.card, state.ZBattlefield)
		granted := false
		for _, ce := range e.active() {
			if ce.MayLookAt && ce.AffectedZone == "Hand" {
				granted = true
			}
		}
		if !granted {
			t.Fatalf("%s: precondition: no live MayLookAt grant over AffectedZone$ Hand", tc.card)
		}
		if e.LibraryTopRevealed(tc.target) || e.MayLookAtLibraryTop(tc.target) {
			t.Errorf("%s: seq %d: a hand-reveal grant covers seat %d's library top", tc.card, len(e.L.Events), tc.target)
		}
		if lt := view.Project(e.G, e, tc.seat, e.Pending()).Players[tc.target].LibraryTop; lt != nil {
			t.Errorf("%s: seq %d: seat %d sees seat %d's library top through a hand reveal: %+v", tc.card, len(e.L.Events), tc.seat, tc.target, lt)
		}
	}
}
