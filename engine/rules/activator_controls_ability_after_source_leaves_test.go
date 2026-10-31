package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestStolenSourceAbilityResolvesUnderItsActivator pins CR 113.8 / 602.2: an
// activated ability is controlled by the player who activated it, whatever
// later happens to its source. Squad Rallier is p0's, stolen by p1, who
// activates its Dig; with the ability on the stack the Rallier leaves the
// battlefield (the sacrifice shape), and the Dig must still look at p1's
// library, not at the card owner's (dirac K1m pairs 39244 92575).
func TestStolenSourceAbilityResolvesUnderItsActivator(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Squad Rallier"))
	rallier := e.G.Zone(state.ZHand, 0)[0]
	// A Grizzly Bears (a creature of power 2) tops each library, so the Dig
	// has something to offer from whichever library it looks at.
	var bears [2]state.ObjID
	for p := state.PlayerID(0); p < 2; p++ {
		o := e.G.AddObject(corpusAlternativeCard(t, "Grizzly Bears"), p)
		o.Zone = state.ZLibrary
		bears[p] = o.ID
		e.G.SetZone(state.ZLibrary, p, append([]state.ObjID{o.ID}, e.G.Zone(state.ZLibrary, p)...))
	}
	placeOnBattlefield(t, e, rallier)
	e.emit(events.Event{Kind: events.ControlChange, Obj: rallier, Player: 1})
	e.G.Active, e.G.Priority = 1, 1
	e.emit(events.Event{Kind: events.ManaAdd, Player: 1, Counter: "W", Amount: 1})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 1, Counter: "C", Amount: 2})
	e.pending = nil
	e.priorityRound()
	submitChoices(t, e, abilityOption(t, e, rallier, 0).Index)
	// The ability is on the stack under p1; its source now leaves.
	if len(e.G.Stack) == 0 {
		t.Fatalf("the activation did not reach the stack; pending %+v", e.Pending())
	}
	if got := e.G.Obj(e.G.Stack[len(e.G.Stack)-1]).Controller; got != 1 {
		t.Fatalf("the activated ability's controller = %d, want 1", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: rallier, From: state.ZBattlefield, To: state.ZGraveyard})
	lib0 := len(e.G.Zone(state.ZLibrary, 0))
	e.resolveTop()
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != lib0 {
		t.Errorf("seat 0's library changed %d -> %d: the Dig looked at the card owner's library", lib0, got)
	}
	if o := e.G.Obj(bears[1]); o.Zone != state.ZHand {
		t.Errorf("seat 1's Bears ended in %v, want the activator's hand (the Dig's default takes the first match)", o.Zone)
	}
}
