package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The `Defined$`-named hidden-library mover (moveDefinedLibraryObjects) routes
// its per-object movement through the shared settle helper, which did not
// carry the `Imprint$ True` persistent association the library-search mover
// did. Dakra Mystic's DBPutRevealed is the real carrier:
//
//	SVar:DBPutRevealed:DB$ ChangeZone | Defined$ Remembered | Origin$ Library
//	  | Destination$ Graveyard | Optional$ True | Imprint$ True | SubAbility$ DBDraw
//	SVar:DBDraw:DB$ Draw | Defined$ Player | ConditionDefined$ Imprinted
//	  | ConditionPresent$ Card | ConditionCompare$ EQ0 | SubAbility$ DBCleanup
//
// The follow-up draw is gated on the source having imprinted NOTHING
// (EQ0). Before the fix the moved card never reached the source's Imprinted
// association, the gate read an empty list and the controller drew extra
// cards. Forge records every card a ChangeZone moved through `Imprint$ True`
// in the source card's imprintedCards list, whatever the destination, and a
// later `Defined$ Imprinted` reads it back; the association is durable state,
// so it must ride an `events.Imprint` folded by events.Apply.
//
// This is a real compiled-corpus regression: the SA, its `Imprint$ True`
// parameter and the EQ0 gate all come from `.cards/cardsfolder/d/dakra_mystic.txt`.

// dakraPutRevealed loads Dakra Mystic's compiled DBPutRevealed continuation
// and asserts it is the direct `Defined$ Remembered | Origin$ Library |
// Destination$ Graveyard | Imprint$ True` mover this test is about -- so a
// corpus or script drift fails loudly instead of silently testing nothing.
func dakraPutRevealed(t *testing.T) (*cards.Card, *cards.SA) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	if reg == nil {
		t.Skip("Dakra Mystic corpus unavailable")
	}
	card, ok := reg.Lookup("Dakra Mystic")
	if !ok || len(card.Faces) == 0 {
		t.Fatal("Dakra Mystic is absent from the corpus")
	}
	sa := cards.ResolveSVar(card.Faces[0].SVars, "DBPutRevealed")
	if sa == nil {
		t.Fatal("Dakra Mystic has no compiled DBPutRevealed continuation")
	}
	if sa.API != "ChangeZone" || sa.Params["Defined"] != "Remembered" ||
		sa.Params["Origin"] != "Library" || sa.Params["Destination"] != "Graveyard" ||
		sa.Params["Imprint"] != "True" {
		t.Fatalf("DBPutRevealed = api=%q defined=%q origin=%q destination=%q imprint=%q, want the direct Defined$ library fetch with Imprint$ True",
			sa.API, sa.Params["Defined"], sa.Params["Origin"], sa.Params["Destination"], sa.Params["Imprint"])
	}
	return card, sa
}

// TestDefinedLibraryFetchImprintsMovedCards is the Dakra Mystic regression:
// a `Defined$ Remembered` card fetched out of a library to the graveyard must
// be recorded in the source's persistent Imprinted association through a
// plain (no-Text) events.Imprint, so the follow-up `ConditionDefined$
// Imprinted ... EQ0` gate no longer reads the empty list.
func TestDefinedLibraryFetchImprintsMovedCards(t *testing.T) {
	card, put := dakraPutRevealed(t)
	// The gate this whole bug is about: `ConditionDefined$ Imprinted |
	// ConditionPresent$ Card | ConditionCompare$ EQ0`. Built from the real
	// DBDraw so the asserted condition is the script's own.
	dbDraw := cards.ResolveSVar(card.Faces[0].SVars, "DBDraw")
	if dbDraw == nil || dbDraw.Params["ConditionDefined"] != "Imprinted" ||
		dbDraw.Params["ConditionCompare"] != "EQ0" {
		t.Fatalf("DBDraw = %+v, want the real Imprinted EQ0 gate", dbDraw)
	}

	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	// The "revealed top card": owned by seat 0 and sitting in seat 0's
	// library -- the two facts the direct fetch rechecks before it moves.
	revealed := h.g.AddObject(mkCard(t, "Name:Revealed Card\nTypes:Sorcery\nOracle:x\n"), 0)
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{revealed.ID})
	if got := h.g.Obj(revealed.ID).Zone; got != state.ZLibrary {
		t.Fatalf("precondition: revealed card zone=%s, want library", got)
	}
	ctx := &Ctx{Source: src.ID, Controller: 0,
		Remembered: []state.Target{{Obj: revealed.ID}}}

	// The gate is a precondition too: with nothing imprinted yet, EQ0 is
	// genuinely satisfied, so the test's post-move "no longer EQ0" assertion
	// is comparing two different values.
	if met, resolved := conditionMet(h, ctx, dbDraw); !resolved || !met {
		t.Fatalf("precondition: Imprinted EQ0 before the fetch met=%v resolved=%v, want true true (empty association)", met, resolved)
	}

	// Run the real DBPutRevealed mover (the registered `ChangeZone`
	// implementation). Its SubAbility chain is deliberately not walked here:
	// DBCleanup's ClearImprinted$ True would legitimately clear the
	// association before the chain ends, and the assertion is about the
	// association the follow-up DBDraw gate reads at resolution time.
	// Optional$ True has no decision channel on this host, so it takes the
	// deterministic "yes" (R-9).
	effChangeZone(h, ctx, put)

	o := h.g.Obj(revealed.ID)
	if o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("fetch left the card %+v, want it in the graveyard", o)
	}
	// Durable association on the SOURCE, folded from events.Imprint.
	srcObj := h.g.Obj(src.ID)
	if srcObj == nil {
		t.Fatal("precondition: the fetch source vanished")
	}
	if !containsID(srcObj.Imprinted, revealed.ID) {
		t.Fatalf("source Imprinted=%v, want it to contain the moved fetch %d", srcObj.Imprinted, revealed.ID)
	}
	// Exactly one plain events.Imprint on the source, carrying the moved card.
	if got := imprintEventCards(h.log, src.ID); len(got) != 1 || got[0] != revealed.ID {
		t.Fatalf("plain events.Imprint ids = %v, want [%d] on source %d", got, revealed.ID, src.ID)
	}
	// The follow-up ability really moved the gate: with the card now
	// imprinted, the EQ0 gate is resolved-false, so DBDraw would be skipped.
	// Asserting on conditionMet (not merely the event) is what ties the
	// association to the draw suppression the bug report describes.
	if met, resolved := conditionMet(h, ctx, dbDraw); !resolved || met {
		t.Fatalf("Imprinted EQ0 after the fetch met=%v resolved=%v, want false true (one imprinted card)", met, resolved)
	}
}
