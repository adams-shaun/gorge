package rules

// RearrangeTopOfLibrary's chooser (cr-g46). "Look at the top three cards of
// target player's library, then put them back in any order. You may have
// that player shuffle." (Natural Selection, Portent): the player who looks,
// orders and decides the shuffle is the spell's controller -- "you" -- not
// the owner of the library (CR 109.5: "you" is the controller). Tahngarth's
// Glare's second half names its chooser explicitly: "That player looks at
// the top three cards of your library, then puts them back in any order"
// (`Defined$ You | RearrangePlayer$ Targeted`).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCRRearrangeTargetLibraryCasterChooses: seat 0 casts Natural Selection
// targeting seat 1. The arrange ask goes to seat 0 over seat 1's top three,
// the answer reorders seat 1's library, and the may-shuffle ask goes to
// seat 0 and shuffles seat 1's library.
func TestCRRearrangeTargetLibraryCasterChooses(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 96, corpusCardText(t, "n/natural_selection.txt"))
	addMana(t, e, 0, "G")
	top := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 1)[:3]...)
	d := castFixture(t, e, id, 1)
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected the KArrange ask, got %+v", d)
	}
	if d.Player != 0 {
		t.Fatalf("CR 109.5: the arrange ask over target player's library went to seat %d, want the caster (seat 0)", d.Player)
	}
	for i, o := range d.Options {
		if o.Obj != top[i] {
			t.Fatalf("option %d = obj %d, want seat 1's library card %d", i, o.Obj, top[i])
		}
	}
	submitChoices(t, e, 2, 1, 0)
	lib := e.G.Zone(state.ZLibrary, 1)
	if lib[0] != top[2] || lib[1] != top[1] || lib[2] != top[0] {
		t.Fatalf("seat 1's top three = %v, want %v reversed", lib[:3], top)
	}
	sd := e.Pending()
	if sd == nil || sd.ResumeKind != "arrange_mayshuffle" {
		t.Fatalf("expected the may-shuffle ask, got %+v", sd)
	}
	if sd.Player != 0 {
		t.Fatalf("\"You may have that player shuffle\": the shuffle ask went to seat %d, want seat 0", sd.Player)
	}
	yes := -1
	for _, o := range sd.Options {
		if o.Kind == "yes" {
			yes = o.Index
		}
	}
	logBefore := len(e.L.Events)
	submitChoices(t, e, yes)
	shuffled := -1
	for _, ev := range e.L.Events[logBefore:] {
		if ev.Kind == events.Shuffle {
			shuffled = int(ev.Player)
		}
	}
	if shuffled != 1 {
		t.Fatalf("the shuffle shuffled seat %d's library, want seat 1's", shuffled)
	}
}

// TestCRRearrangePlayerNamesTheChooser: Tahngarth's Glare cast by seat 0 at
// seat 1. The first arrange (seat 1's library) is seat 0's; the second
// (seat 0's library, RearrangePlayer$ Targeted) is seat 1's.
func TestCRRearrangePlayerNamesTheChooser(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 96, corpusCardText(t, "t/tahngarths_glare.txt"))
	addMana(t, e, 0, "R")
	d := castFixture(t, e, id, 1)
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected the first KArrange ask, got %+v", d)
	}
	if d.Player != 0 || e.G.Obj(d.Options[0].Obj).Owner != 1 {
		t.Fatalf("first arrange: player %d over seat %d's library, want seat 0 over seat 1's",
			d.Player, e.G.Obj(d.Options[0].Obj).Owner)
	}
	submitChoices(t, e, 0, 1, 2)
	d2 := e.Pending()
	if d2 == nil || d2.Kind != decision.KArrange {
		t.Fatalf("expected the second KArrange ask, got %+v", d2)
	}
	if d2.Player != 1 || e.G.Obj(d2.Options[0].Obj).Owner != 0 {
		t.Fatalf("RearrangePlayer$ Targeted: second arrange is seat %d over seat %d's library, want seat 1 over seat 0's",
			d2.Player, e.G.Obj(d2.Options[0].Obj).Owner)
	}
	top := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)[:3]...)
	submitChoices(t, e, 1, 2, 0)
	lib := e.G.Zone(state.ZLibrary, 0)
	if lib[0] != top[1] || lib[1] != top[2] || lib[2] != top[0] {
		t.Fatalf("seat 0's top three = %v, want seat 1's order [%d %d %d]", lib[:3], top[1], top[2], top[0])
	}
}

// TestCRRearrangeOwnLibraryUnchanged guards the Defined$ You population
// (Ponder): the ask stays the caster's over the caster's own library.
func TestCRRearrangeOwnLibraryUnchanged(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 96, ponderSrc)
	addMana(t, e, 0, "U")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KArrange || d.Player != 0 || e.G.Obj(d.Options[0].Obj).Owner != 0 {
		t.Fatalf("Ponder's arrange must be seat 0's over its own library, got %+v", d)
	}
}
