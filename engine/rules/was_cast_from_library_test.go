package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCastOriginAdmitsYourLibrary pins this: the
// wasCastFromYourLibrary origin token (Melek, Izzet Paragon's copy trigger,
// Wickerwing Effigy's Bird grant) is stripped by the rules-side origin chain
// and holds only for a library cast made by the evaluating you. Outside
// castOriginTokens it reaches the effects filter unstripped and fails closed,
// so Melek never copies a spell cast off the library top.
func TestCastOriginAdmitsYourLibrary(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, flashFixture))
	id := e.G.Zone(state.ZHand, 0)[0]
	if _, ok := e.castProvenanceAdmits("Instant.wasCastFromYourLibrary", id, 0); ok {
		t.Fatal("an uncast card must not read as cast from your library")
	}
	e.emit(events.Event{Kind: events.PutOnStack, Obj: id, Player: 0,
		From: state.ZLibrary, To: state.ZStack, Text: "Flash"})
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZStack || o.LatestCastFrom != state.ZLibrary {
		t.Fatalf("precondition: want a library-origin cast on the stack, got %+v", o)
	}
	spec := "Instant.wasCastFromYourLibrary,Sorcery.wasCastFromYourLibrary"
	if s, ok := e.castProvenanceAdmits(spec, id, 0); !ok || s != "Instant,Sorcery" {
		t.Fatalf("library-origin spec did not strip for its caster (got %q, ok %v)", s, ok)
	}
	if _, ok := e.castProvenanceAdmits(spec, id, 1); ok {
		t.Fatal("seat 1's YourLibrary read must not hold on seat 0's library cast")
	}
	// A hand-origin cast never reads as a library cast.
	e2 := handEngine(t, card(t, flashFixture))
	id2 := e2.G.Zone(state.ZHand, 0)[0]
	e2.emit(events.Event{Kind: events.PutOnStack, Obj: id2, Player: 0,
		From: state.ZHand, To: state.ZStack, Text: "Flash"})
	if _, ok := e2.castProvenanceAdmits(spec, id2, 0); ok {
		t.Fatal("a hand-origin cast must not read as cast from your library")
	}
}
