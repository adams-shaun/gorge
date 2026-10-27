package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// dinasGuidancePick casts Dina's Guidance, answers the search with the first
// offered creature and returns the engine, its config and the pick.
func dinasGuidancePick(t *testing.T) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Dina's Guidance", "Grizzly Bears")
	id := searchMoveByName(t, e, "Dina's Guidance", state.ZHand)
	addMana(t, e, 0, "BGC")
	submitChoices(t, e, plainCastOption(t, e, id).Index)
	passUntilNonPriority(t, e, 30)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search" {
		t.Fatalf("pending = %+v, want a search KChoose", d)
	}
	picked := d.Options[0].Obj
	if z := e.G.Obj(picked).Zone; z != state.ZLibrary {
		t.Fatalf("precondition: picked card in %s, want library", z)
	}
	submitChoices(t, e, 0)
	return e, cfg, picked
}

func dinasGuidanceDestAsk(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "changezone_dest_alt" || len(d.Options) != 2 {
		t.Fatalf("pending = %+v, want a changezone_dest_alt KChoose with two options", d)
	}
	if d.Options[0].Label != "hand" || d.Options[1].Label != "graveyard" {
		t.Fatalf("destination options = %q, %q, want hand, graveyard", d.Options[0].Label, d.Options[1].Label)
	}
}

func TestDinasGuidanceGraveyardChoice(t *testing.T) {
	t.Parallel()
	e, cfg, picked := dinasGuidancePick(t)
	dinasGuidanceDestAsk(t, e)
	submitChoices(t, e, 1)
	if got := e.G.Obj(picked).Zone; got != state.ZGraveyard {
		t.Fatalf("creature zone = %s, want graveyard", got)
	}
	replayCheck(t, e, cfg)
}

func TestDinasGuidanceHandChoice(t *testing.T) {
	t.Parallel()
	e, cfg, picked := dinasGuidancePick(t)
	dinasGuidanceDestAsk(t, e)
	submitChoices(t, e, 0)
	if got := e.G.Obj(picked).Zone; got != state.ZHand {
		t.Fatalf("creature zone = %s, want hand", got)
	}
	replayCheck(t, e, cfg)
}

func TestDinasGuidanceFailToFindAsksNothing(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Dina's Guidance")
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...) {
		if o := e.G.Obj(id); o.Face().Name == "Grizzly Bears" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZGraveyard})
		}
	}
	id := searchMoveByName(t, e, "Dina's Guidance", state.ZHand)
	for _, lid := range e.G.Zone(state.ZLibrary, 0) {
		if n := e.G.Obj(lid).Face().Name; n == "Grizzly Bears" {
			t.Fatalf("precondition: a creature remains in the library")
		}
	}
	addMana(t, e, 0, "BGC")
	submitChoices(t, e, plainCastOption(t, e, id).Index)
	if n := passUntilStackEmpty(t, e, 30); len(e.G.Stack) != 0 {
		t.Fatalf("stack not drained after %d passes", n)
	}
	if d := e.Pending(); d != nil && d.ResumeKind == "changezone_dest_alt" {
		t.Fatalf("fail-to-find posed a destination ask: %+v", d)
	}
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("Dina's Guidance zone = %s, want graveyard (resolved)", z)
	}
}
