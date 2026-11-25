package rules

// CR 726.2: "Whenever a player takes the initiative, that player ventures
// into Undercity" is an inherent TRIGGERED ability, not part of the
// TakeInitiative effect's own resolution. These tests drive the ETB through
// the real priority rounds and require the venture to reach the stack as its
// own object before the Undercity's Secret Entrance room is entered.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// passInitiativePriorityOnce answers the pending priority decision with "pass".
func passInitiativePriorityOnce(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected a priority decision to pass, got %+v", d)
	}
	submitChoices(t, e, highestPriorityOptionWithKind(t, e, "pass").Index)
}

// passInitiativePriorityRound passes for every living seat, resolving the top stack
// object (CR 117.4) and returning at the next priority window.
func passInitiativePriorityRound(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < e.G.AliveCount(); i++ {
		passInitiativePriorityOnce(t, e)
	}
}

// TestTakeInitiativeVentureUsesTheStack is the CR 726.2 shape: Avenging
// Hunter's TakeInitiative ETB resolves, the initiative moves, but its
// venture is a separate triggered ability waiting on the stack -- the
// Undercity is NOT entered yet. Only the next pass pair resolves that
// trigger and enters Secret Entrance, whose room ability then resolves the
// basic-land search.
func TestTakeInitiativeVentureUsesTheStack(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	hunter := mustCorpusCard(t, reg, "Avenging Hunter")
	e, cfg := initiativeEngine(t, reg, []*cards.Card{hunter, hunter}, nil)

	handBefore := len(e.G.Zone(state.ZHand, 0))
	if handBefore != 7 {
		t.Fatalf("precondition: opening hand holds %d cards, want 7", handBefore)
	}
	id := moveByName(t, e, 0, "Avenging Hunter", state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Avenging Hunter not on the battlefield: %+v", o)
	}
	handAfterMove := len(e.G.Zone(state.ZHand, 0))
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("precondition: Avenging Hunter ETB queued %d triggers, want 1", len(e.pendingTriggers))
	}
	if e.G.HasInitiative {
		t.Fatalf("precondition: the game began with the initiative (who=%d)", e.G.Initiative)
	}
	if e.G.Players[0].DungeonObj != 0 {
		t.Fatalf("precondition: seat 0 already owns dungeon %d", e.G.Players[0].DungeonObj)
	}

	// Place the ETB on the stack and hold priority with it there.
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision with the ETB on the stack: %+v", d)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack %v, want the ETB trigger alone", e.G.Stack)
	}
	top := e.G.Obj(e.G.Stack[0])
	if top == nil || top.Ability == nil || top.Ability.API != "TakeInitiative" {
		t.Fatalf("top of stack is not the TakeInitiative ETB: %+v", top)
	}

	// Resolve the ETB. The designation lands, but the CR 726.2 venture must
	// be its own object on the stack; the Undercity must still be unentered.
	passInitiativePriorityRound(t, e)
	if !e.G.IsInitiative(0) {
		t.Fatalf("the ETB did not give seat 0 the initiative: has=%v who=%d", e.G.HasInitiative, e.G.Initiative)
	}
	if got := len(e.G.Stack); got != 1 {
		t.Fatalf("after the ETB resolved the stack holds %d objects, want the one CR 726.2 venture trigger", got)
	}
	top = e.G.Obj(e.G.Stack[0])
	if top == nil || top.Ability == nil || top.Ability.API != "Venture" || top.Source != 0 {
		t.Fatalf("top of stack is not the source-less CR 726.2 venture trigger: %+v", top)
	}
	if e.G.Players[0].DungeonObj != 0 {
		t.Fatal("the TakeInitiative resolution folded the venture inline: the Undercity was entered before the trigger resolved")
	}
	if n := initiativeVentureEvents(e, 0); n != 1 {
		t.Fatalf("logged %d CR 726.2 venture pushes, want the one on the stack", n)
	}

	// One more pass pair resolves the venture: Secret Entrance is entered and
	// its room ability sits on the stack with the basic-land search not yet
	// resolved (the hand is still the dealt seven).
	passInitiativePriorityRound(t, e)
	dobj, room := dungeonRoom(t, e, 0)
	if got := e.G.Obj(dobj).Face().Name; got != "Undercity" {
		t.Fatalf("entered dungeon %q, want Undercity", got)
	}
	if room != "Entrance" {
		t.Fatalf("marker on %q, want Undercity's first room Entrance", room)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handAfterMove {
		t.Fatalf("hand holds %d cards with the room ability still on the stack, want %d (the search has not resolved)", got, handAfterMove)
	}
	if got := len(e.G.Stack); got != 1 {
		t.Fatalf("the room ability is not the one object on the stack: %d objects", got)
	}

	drainInitiative(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)); got != handAfterMove+1 {
		t.Fatalf("Secret Entrance's search left %d cards in hand, want %d (the pre-move hand plus one land)", got, handAfterMove+1)
	}

	// CR 726.5: taking it again while already the holder still ventures. A
	// second Avenging Hunter's ETB takes the designation again; the
	// designation does not move, but its source-less trigger is queued and
	// resolves, moving the marker from Entrance along its first arrow.
	before := initiativeVentureEvents(e, 0)
	second := moveByName(t, e, 0, "Avenging Hunter", state.ZBattlefield)
	if o := e.G.Obj(second); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the second Avenging Hunter is not on the battlefield: %+v", o)
	}
	if !e.G.IsInitiative(0) {
		t.Fatal("precondition: seat 0 lost the initiative before the retake")
	}
	drainInitiative(t, e)
	if n := initiativeVentureEvents(e, 0); n != before+1 {
		t.Fatalf("the CR 726.5 retake pushed %d ventures, want %d", n, before+1)
	}
	if _, room := dungeonRoom(t, e, 0); room != "Forge" {
		t.Fatalf("retake marker %q, want Forge (Secret Entrance's first printed arrow)", room)
	}
	initiativeReplayHead(t, e, cfg)
}
