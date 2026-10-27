package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// tokenEntryBoard puts the named corpus permanent onto seat 0's battlefield
// and mints one 1/1 Goblin token for seat `owner` through the ordinary
// TokenCreate emit, returning the token.
func tokenEntryBoard(t *testing.T, permanent string, owner state.PlayerID) (*Engine, *state.Object) {
	t.Helper()
	e := handEngineTokens(t, corpusAlternativeCard(t, permanent))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	drainQueuedTriggers(t, e)
	want := e.G.NextID
	e.EmitTokenCreate(events.Event{Kind: events.TokenCreate, Player: owner, Text: "r_1_1_goblin"})
	drainQueuedTriggers(t, e)
	tok := e.G.Obj(want)
	if tok == nil || tok.Zone != state.ZBattlefield || !tok.IsToken {
		t.Fatalf("precondition: the Goblin token was not created: %+v", tok)
	}
	return e, tok
}

// TestAuthorityOfTheConsulsTapsOpponentsTokens: a token created onto the
// battlefield enters it, so "Creatures your opponents control enter tapped"
// taps an opponent's creature token (CR 111.1, 614.1c) and leaves its
// controller's own token untapped. The scripted mint has no MoveZone, so the
// Moved replacement never matched it before.
func TestAuthorityOfTheConsulsTapsOpponentsTokens(t *testing.T) {
	t.Parallel()
	_, tok := tokenEntryBoard(t, "Authority of the Consuls", 1)
	if !tok.Tapped {
		t.Fatal("the opponent's Goblin token entered untapped under Authority of the Consuls")
	}
	_, own := tokenEntryBoard(t, "Authority of the Consuls", 0)
	if own.Tapped {
		t.Fatal("Authority of the Consuls tapped its controller's own token")
	}
}

// TestGrumgullyCountersOwnCreatureTokens: the K:ETBReplacement:Other
// "enters with an additional +1/+1 counter" shape reaches a creature token
// its controller creates, and not an opponent's.
func TestGrumgullyCountersOwnCreatureTokens(t *testing.T) {
	t.Parallel()
	_, tok := tokenEntryBoard(t, "Grumgully, the Generous", 0)
	if got := tok.Counter("P1P1"); got != 1 {
		t.Fatalf("own Goblin token +1/+1 counters = %d, want 1", got)
	}
	_, theirs := tokenEntryBoard(t, "Grumgully, the Generous", 1)
	if got := theirs.Counter("P1P1"); got != 0 {
		t.Fatalf("opponent's Goblin token +1/+1 counters = %d, want 0", got)
	}
}

// hallowedMoonlightBoard casts Hallowed Moonlight from seat 0's hand and
// resolves it, returning the engine with its "Moved" replacement active.
func hallowedMoonlightBoard(t *testing.T) *Engine {
	t.Helper()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Hallowed Moonlight"))
	hm := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MW], e.G.Players[0].Pool[state.MC] = 1, 1
	castMode(t, e, hm, "")
	e.resolveTop()
	drainQueuedTriggers(t, e)
	if o := e.G.Obj(hm); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Hallowed Moonlight not in the graveyard: %+v", o)
	}
	found := false
	for _, ce := range e.active() {
		if ce.ReplacementEvent == "Moved" && ce.ReplacementParam(cards.PKReplacementResult) == "" {
			with := replacementBodySA(ce.ReplacementBody)
			found = with != nil && with.APIKind() == cards.APIChangeZone &&
				tokenEntryBody("", ce.ReplacementParam(cards.PKDestination), with)
		}
	}
	if !found {
		t.Fatal("precondition: no active full Moved replacement with a ChangeZone body")
	}
	return e
}

// TestHallowedMoonlightExilesOpponentToken: a creature token an opponent
// creates is exiled instead of entering the battlefield (CR 614.1, 111.1).
func TestHallowedMoonlightExilesOpponentToken(t *testing.T) {
	t.Parallel()
	e := hallowedMoonlightBoard(t)
	want := e.G.NextID
	e.EmitTokenCreate(events.Event{Kind: events.TokenCreate, Player: 1, Text: "r_1_1_goblin"})
	drainQueuedTriggers(t, e)
	tok := e.G.Obj(want)
	if tok == nil || !tok.IsToken {
		t.Fatalf("precondition: the Goblin token was not created: %+v", tok)
	}
	if tok.Zone != state.ZExile {
		t.Fatalf("opponent's Goblin token is in %s under Hallowed Moonlight, want Exile", tok.Zone)
	}
}

// TestHallowedMoonlightSuppressesTokenEntryTriggers: a token exiled instead
// of entering raises no "enters the battlefield" trigger.
func TestHallowedMoonlightSuppressesTokenEntryTriggers(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Hallowed Moonlight"))
	warden := e.G.AddObject(corpusAlternativeCard(t, "Soul Warden"), 0)
	warden.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{warden.ID})
	soulWardenTrigger(t, e, warden.ID)
	hm := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MW], e.G.Players[0].Pool[state.MC] = 1, 1
	castMode(t, e, hm, "")
	e.resolveTop()
	drainQueuedTriggers(t, e)
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("precondition: seat 0 life = %d, want 20", got)
	}
	want := e.G.NextID
	e.EmitTokenCreate(events.Event{Kind: events.TokenCreate, Player: 1, Text: "r_1_1_goblin"})
	drainQueuedTriggers(t, e)
	if tok := e.G.Obj(want); tok == nil || tok.Zone != state.ZExile {
		t.Fatalf("precondition: the Goblin token was not exiled: %+v", tok)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("seat 0 life = %d, want 20 (no entry trigger for an exiled token)", got)
	}
}
