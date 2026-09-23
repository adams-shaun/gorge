package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestScryTriggerFromLostIsleCalling(t *testing.T) {
	e, _, id := scryFixture(t, 8211)
	source := onBoardCard(t, e, 0, corpusCard(t, "Lost Isle Calling"))
	if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield || o.Face().Triggers[0].Mode != "Scry" {
		t.Fatalf("precondition: Scry trigger source not on battlefield: %+v", o)
	}
	if n := e.G.Obj(source).Counter("Verse"); n != 0 {
		t.Fatalf("precondition: verse counter = %d, want 0", n)
	}
	d := scryDecision(t, e, id)
	if d.Kind != decision.KArrange || len(d.Options) != 3 {
		t.Fatalf("scry decision = %+v", d)
	}
	found := false
	for _, pt := range e.pendingTriggers {
		if pt.Source == source {
			found = true
		}
	}
	if !found {
		t.Fatalf("Mode$ Scry was not matched/queued: %+v", e.pendingTriggers)
	}
	submitChoices(t, e, 0, 1, 2)
	passUntilStackEmpty(t, e, 20)
	if n := e.G.Obj(source).Counter("Verse"); n != 1 {
		t.Fatalf("resolved Scry trigger verse counters = %d, want 1", n)
	}
}

func TestKenessosReplacesScryCountBeforeLooking(t *testing.T) {
	e, _, id := scryFixture(t, 8212)
	source := onBoardCard(t, e, 0, corpusCard(t, "Kenessos, Priest of Thassa"))
	if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield || o.Face().Repls[0].Event != "Scry" {
		t.Fatalf("precondition: Scry replacement not on battlefield: %+v", o)
	}
	d := scryDecision(t, e, id)
	if len(e.G.Zone(state.ZLibrary, 0)) < 4 {
		t.Fatal("precondition: library has fewer than four cards")
	}
	if d.Max != 4 || len(d.Options) != 4 {
		t.Fatalf("Kenessos Scry 3 arranged %d options (max %d), want 4", len(d.Options), d.Max)
	}
	found := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Scry && ev.Obj == id && ev.Player == 0 {
			found = true
			if ev.Amount != 4 {
				t.Fatalf("logged replaced Scry count = %d, want 4", ev.Amount)
			}
		}
	}
	if !found {
		t.Fatal("no Scry instruction recorded")
	}
}

func TestEligethDrawsInsteadOfScrying(t *testing.T) {
	e, _, id := scryFixture(t, 8213)
	source := onBoardCard(t, e, 0, corpusCard(t, "Eligeth, Crossroads Augur"))
	if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield || o.Face().Repls[0].Event != "Scry" {
		t.Fatalf("precondition: Scry replacement not on battlefield: %+v", o)
	}
	before := len(e.G.Zone(state.ZHand, 0))
	if len(e.G.Zone(state.ZLibrary, 0)) < 3 {
		t.Fatal("precondition: library has fewer than three cards")
	}
	castFromPriority(t, e, id)
	passUntilStackEmpty(t, e, 20)
	if d := e.Pending(); d != nil && d.Kind == decision.KArrange {
		t.Fatalf("replaced scry asked for arrangement: %+v", d)
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - before; got != 2 {
		// The spell left the hand (-1); Eligeth drew three (+3).
		t.Fatalf("net hand change = %d, want +2 (cast one, draw three)", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Scry && ev.Obj == id {
			t.Fatal("replaced scry logged an instruction and may fire Scry triggers")
		}
	}
}
