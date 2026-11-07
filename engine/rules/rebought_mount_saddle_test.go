package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestBecomesSaddledFirstTimeAfterZoneChange: a Stubborn Burrowfiend saddled,
// returned to hand and put back onto the battlefield in one turn is a new
// object (CR 400.7), so its next saddle is that object's first this turn and
// the FirstTimeSaddled$ trigger fires again. The Mill 2 body is the witness:
// two saddles across the zone change mill four cards, not two.
func TestBecomesSaddledFirstTimeAfterZoneChange(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	burrow := corpusAlternativeCard(t, "Stubborn Burrowfiend")
	e := handEngine(t)
	mount := addToBattlefield(t, e, burrow, 0)
	bears := make([]state.ObjID, 4)
	for i := range bears {
		bears[i] = addToBattlefield(t, e, lookup(t, reg, "Runeclaw Bear"), 0)
		e.G.Obj(bears[i]).SummonSick = false
	}
	e.G.Obj(mount).SummonSick = false
	if n := len(e.G.Zone(state.ZLibrary, 0)); n < 4 {
		t.Fatalf("precondition: library holds %d cards, want >= 4 for two Mill 2s", n)
	}
	before := len(e.G.Zone(state.ZGraveyard, 0))

	saddleWith := func(first, second state.ObjID) {
		t.Helper()
		d := saddleElection(t, e, mount, 2)
		var choices []int
		for _, o := range d.Options {
			if o.Obj == first || o.Obj == second {
				choices = append(choices, o.Index)
			}
		}
		if len(choices) != 2 {
			t.Fatalf("saddle election offered %d of the two intended bears: %+v", len(choices), d.Options)
		}
		submitChoices(t, e, choices...)
		passUntilStackEmpty(t, e, 40)
	}
	saddleWith(bears[0], bears[1])
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != before+2 {
		t.Fatalf("precondition: first saddle grew the graveyard by %d, want 2 (one Mill 2)", got-before)
	}

	// Bounce and replay the Mount within the same turn.
	turn := e.G.Turn
	e.emit(events.Event{Kind: events.MoveZone, Obj: mount, From: state.ZBattlefield, To: state.ZHand})
	e.emit(events.Event{Kind: events.MoveZone, Obj: mount, From: state.ZHand, To: state.ZBattlefield})
	if o := e.G.Obj(mount); o == nil || o.Zone != state.ZBattlefield || e.G.Turn != turn {
		t.Fatal("precondition: the Mount is not back on the battlefield in the same turn")
	}
	e.G.Obj(mount).SummonSick = false

	saddleWith(bears[2], bears[3])
	if got := e.G.Obj(mount).SaddledTurn; got != e.G.Turn {
		t.Fatalf("precondition: second saddle did not stamp SaddledTurn (got %d, turn %d)", got, e.G.Turn)
	}
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != before+4 {
		t.Fatalf("CR 400.7: graveyard grew by %d across the two saddles, want 4 (the new object's first saddle fires)", got-before)
	}
}
