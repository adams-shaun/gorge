package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestSynthEradicatorRememberPutGatesTheExiledCard pins the real attack
// trigger's chained DBPlay. Declining the energy election leaves no remembered
// player, so the EQ0 condition registers the exile may-play grant; accepting
// remembers the player recipient and suppresses that grant.
func TestSynthEradicatorRememberPutGatesTheExiledCard(t *testing.T) {
	run := func(seed uint64, accept bool) (*Engine, state.ObjID) {
		e, _, synth := gateFixture(t, seed, "Synth Eradicator")
		synth = gateMoveFromLibrary(t, e, "Synth Eradicator", state.ZBattlefield)
		if e.G.Obj(synth).Zone != state.ZBattlefield {
			t.Fatal("precondition: Synth Eradicator is not on the battlefield")
		}
		e.emit(events.Event{Kind: events.TriggerPush, Obj: synth, Player: 0})
		e.resolveTop()
		if d := e.Pending(); d == nil || d.ResumeKind != "put_optional" {
			t.Fatalf("attack trigger did not reach the optional energy election: %+v", d)
		}
		answer := 1
		if accept {
			answer = 0
		}
		answerPutOptional(t, e, answer)
		if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
			t.Fatalf("unexpected pending decision after resolving attack trigger: %+v", d)
		}
		var exiled state.ObjID
		for _, id := range e.G.Zone(state.ZExile, 0) {
			if id != synth {
				exiled = id
				break
			}
		}
		if exiled == 0 {
			t.Fatal("precondition: attack trigger did not exile a card")
		}
		return e, exiled
	}

	declined, declinedCard := run(917, false)
	if !synthEradicatorHasPlayGrant(declined, declinedCard) {
		t.Fatalf("declining the energy election did not make exiled card %d playable; active=%+v events=%+v", declinedCard, declined.active(), declined.L.Events)
	}
	accepted, acceptedCard := run(918, true)
	if synthEradicatorHasPlayGrant(accepted, acceptedCard) {
		t.Fatal("accepted energy election still granted play of the exiled card")
	}
}

func synthEradicatorHasPlayGrant(e *Engine, card state.ObjID) bool {
	for _, ce := range e.active() {
		if ce.MayPlay && ce.AffectedZone == "Exile" && objIDIn(ce.Remembered, card) {
			return true
		}
	}
	return false
}
