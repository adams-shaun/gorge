package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Dauthi Voidwalker's "exile it with a void counter on it" leaves the counter
// on the card for as long as it stays exiled (CR 122.1, 400.7): the
// counter the dirac scoping ticket needs a representation for. The scenario
// is the battlefield-origin half of the card (the any-zone half shares the
// ChangeZone body); Draugr Necromancer's ICE counter takes the same path.
func TestDauthiVoidwalkerExilesWithAVoidCounterAndKeepsIt(t *testing.T) {
	t.Parallel()
	dauthi := corpusAlternativeCard(t, "Dauthi Voidwalker")
	bear := card(t, "Name:Opp Bear\nManaCost:0\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, _ := tokenReplGameSeats(t, 7, []*cards.Card{dauthi}, []*cards.Card{bear})
	moveSeededCard(t, e, 0, dauthi, state.ZBattlefield)
	b := moveSeededCardInStep(t, e, 1, bear, state.ZBattlefield)
	if got := e.G.Obj(b).Zone; got != state.ZBattlefield {
		t.Fatalf("precondition: opponent's bear is in zone %v, want the battlefield", got)
	}
	if got := e.G.Obj(b).Counter("VOID"); got != 0 {
		t.Fatalf("precondition: bear already carries %d void counters", got)
	}
	e.G.Obj(b).Damage = 2 // lethal (CR 704.5g)
	e.checkStateBased()
	o := e.G.Obj(b)
	if o.Zone != state.ZExile || o.Counter("VOID") != 1 {
		t.Fatalf("the dying bear is in zone %v with %d void counters, want exile with 1", o.Zone, o.Counter("VOID"))
	}
	if !contains(e.G.Zone(state.ZExile, 1), b) || contains(e.G.Zone(state.ZGraveyard, 1), b) {
		t.Fatal("the bear is not in its owner's exile zone, or also in the graveyard")
	}
	// the counter is a logged event on the exiled object, not a side table
	logged := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == b && ev.Counter == "VOID" && ev.Amount == 1 {
			logged = true
		}
	}
	if !logged {
		t.Fatal("no CounterChange(VOID, +1) event on the exiled bear in the log")
	}
}
