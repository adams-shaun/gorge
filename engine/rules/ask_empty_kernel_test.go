package rules

// The optional direct-library-fetch continuations, answered through the
// resolution kernel on a real engine.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const forestFixtureSrc = "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"

// TestBucolicRanchBottomContinuationKernel: Bucolic Ranch's real optional
// DBChangeZone2 poses a yes/no before moving anything; accepting it puts the
// actual top card of the library on the bottom.
func TestBucolicRanchBottomContinuationKernel(t *testing.T) {
	t.Parallel()
	ranch := corpusCardNamed(t, "Bucolic Ranch")
	bottom := svarSA(t, ranch, "DBChangeZone2")
	e := fixtureEngine(t, 2)
	src := placeCard(t, e, 0, ranch, state.ZBattlefield)
	ids := addToLibrary(t, e, 0, forestFixtureSrc, 3)
	start := len(e.L.Events)
	d := runProbeSA(t, e, bottom, func() *effects.Ctx { return &effects.Ctx{Source: src, Controller: 0} }, nil)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "defined_library_optional" {
		t.Fatalf("DBChangeZone2 posed %+v, want an optional direct-fetch decision", d)
	}
	if n := countEventKind(eventsSince(e, start), events.MoveZone); n != 0 {
		t.Fatalf("DBChangeZone2 moved %d card(s) before its answer", n)
	}
	answerAsk(t, e, optionKindIndex(t, d, "yes"))
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) != len(ids) || lib[len(lib)-1] != ids[0] {
		t.Fatalf("accepted DBChangeZone2 library = %v, want top %d on bottom", lib, ids[0])
	}
	var moved, ordered bool
	for _, ev := range eventsSince(e, start) {
		moved = moved || ev.Kind == events.MoveZone && ev.Obj == ids[0]
		ordered = ordered || ev.Kind == events.LibraryOrder
	}
	if !moved || !ordered {
		t.Fatalf("accepted DBChangeZone2 events = %v, want top-card move and library order", eventsSince(e, start))
	}
}

// TestDefinedLibraryOptionalDeclineLeavesTheFetchListAloneKernel: Kenessos's
// real DBBottom continuation offers yes/no; declining neither moves nor
// shuffles nor reorders the remembered library card.
func TestDefinedLibraryOptionalDeclineLeavesTheFetchListAloneKernel(t *testing.T) {
	t.Parallel()
	kenessos := corpusCardNamed(t, "Kenessos, Priest of Thassa")
	bottom := svarSA(t, kenessos, "DBBottom")
	e := fixtureEngine(t, 2)
	src := placeCard(t, e, 0, kenessos, state.ZBattlefield)
	id := addToLibrary(t, e, 0, "Name:Sea Monster\nTypes:Creature\nPT:1/1\nOracle:x\n", 1)[0]
	start := len(e.L.Events)
	d := runProbeSA(t, e, bottom, func() *effects.Ctx {
		return &effects.Ctx{Source: src, Controller: 0, Remembered: []state.Target{{Obj: id}}}
	}, nil)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "defined_library_optional" {
		t.Fatalf("optional direct fetch asked %+v, want defined_library_optional KChoose", d)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
		t.Fatalf("optional direct fetch options = %+v, want yes/no", d.Options)
	}
	if n := countEventKind(eventsSince(e, start), events.MoveZone); n != 0 {
		t.Fatalf("optional direct fetch moved before its answer")
	}
	answerAsk(t, e, optionKindIndex(t, d, "no"))
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZLibrary {
		t.Fatalf("declined DBBottom left card %+v, want it in the library", o)
	}
	for _, ev := range eventsSince(e, start) {
		if ev.Kind == events.MoveZone || ev.Kind == events.Shuffle || ev.Kind == events.LibraryOrder {
			t.Fatalf("declined DBBottom emitted %v, want no move or reorder", ev)
		}
	}
}
