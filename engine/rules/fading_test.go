package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func blastoderm(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	b, ok := reg.Lookup("Blastoderm")
	if !ok {
		t.Fatal("corpus fixture: Blastoderm missing")
	}
	e := New(Config{Seed: 17, Names: []string{"a", "b"}, Decks: [][]*cards.Card{
		append([]*cards.Card{b}, mountainDeck(t, 39)...), mountainDeck(t, 40),
	}, Tokens: reg.Tokens})
	e.Advance()
	toMain1(t, e)
	id := moveByName(t, e, 0, "Blastoderm", state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Blastoderm is not on the battlefield: %+v", o)
	}
	if got := e.G.Obj(id).Counter("FADE"); got != 3 {
		t.Fatalf("precondition: Blastoderm entered with %d FADE counters, want 3", got)
	}
	e.pendingTriggers = nil
	return e, id
}

func fadingUpkeep(e *Engine, active state.PlayerID) int {
	e.G.Active, e.G.Priority = active, active
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	e.putTriggersOnStack()
	n := len(e.G.Stack)
	for len(e.G.Stack) > 0 {
		e.resolveTop()
	}
	return n
}

// CR 702.32a: three fade counters last three upkeeps; the fourth upkeep finds
// none to remove and sacrifices. The opponent's upkeep does nothing.
func TestFadingBlastodermRemovesThenSacrifices(t *testing.T) {
	t.Parallel()
	e, id := blastoderm(t)
	if n := fadingUpkeep(e, 1); n != 0 || e.G.Obj(id).Counter("FADE") != 3 {
		t.Fatalf("opponent upkeep: %d triggers, FADE %d; want none and 3", n, e.G.Obj(id).Counter("FADE"))
	}
	for _, want := range []int32{2, 1, 0} {
		if n := fadingUpkeep(e, 0); n != 1 {
			t.Fatalf("upkeep put %d triggers, want one", n)
		}
		o := e.G.Obj(id)
		if o.Zone != state.ZBattlefield || o.Counter("FADE") != want {
			t.Fatalf("after upkeep: zone %s FADE %d, want battlefield and %d", o.Zone, o.Counter("FADE"), want)
		}
	}
	if n := fadingUpkeep(e, 0); n != 1 {
		t.Fatalf("fourth upkeep put %d triggers, want one", n)
	}
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("fourth upkeep left Blastoderm in %s, want the graveyard", z)
	}
}
