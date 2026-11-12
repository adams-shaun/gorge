package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The player half of effPump. A Pump whose affected set names a player —
// The Stasis Coffin's and The One Ring's "you gain protection from
// everything until your next turn" (CR 702.16c) — skipped the object walk
// and registered nothing, so the protected seat stayed targetable and
// killable for the whole window. The grant now registers as a continuous
// player keyword static (effects/combatfx.go registerPlayerPumpGrant) that
// rules' playerKeywords surface reads beside every printed
// `Affected$ You | AddKeyword$` static, so the existing player gates —
// playerProtectedFrom (damage, CR 702.16e, and targeting, CR 702.16b) —
// enforce it without new rules code.

// TestStasisCoffinPlayerProtectionUntilNextTurn plays the real corpus
// carrier's activation. The coffin exiles itself as part of the cost, so
// the window is also pinned against the source's departure: a
// resolution-generated continuous effect lasts its stated duration
// (CR 611.2a) and outlives a source that is no longer on the battlefield.
func TestStasisCoffinPlayerProtectionUntilNextTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{
		lookup(t, reg, "The Stasis Coffin"), lookup(t, reg, "Grizzly Bears"),
	}, []*cards.Card{
		lookup(t, reg, "Grizzly Bears"),
	})
	coffin := moveByName(t, e, 0, "The Stasis Coffin", state.ZBattlefield)
	bear0 := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	bear1 := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	if e.playerProtectedFrom(0, bear1) {
		t.Fatal("precondition: seat 0 is already protected from the bear")
	}
	e.pending = nil
	e.priorityRound()
	addMana(t, e, 0, "CC")
	submitChoices(t, e, abilityOption(t, e, coffin, 0).Index)
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(coffin); o == nil || o.Zone != state.ZExile {
		t.Fatalf("precondition: the coffin did not exile itself as the cost: %+v", o)
	}
	if kw := e.playerKeywords(0); !containsKW(kw, "Protection from everything") {
		t.Fatalf("the grant did not register: playerKeywords(0)=%v", kw)
	}
	// CR 702.16e: all damage from any source is prevented, colourless and
	// coloured alike ("everything").
	for _, src := range []state.ObjID{bear1, bear0} {
		if !e.playerProtectedFrom(0, src) {
			t.Fatalf("seat 0 is not protected from source %d", src)
		}
	}
	life := e.G.Players[0].Life
	e.damaging = bear1
	e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 2})
	e.damaging = 0
	if got := e.G.Players[0].Life; got != life {
		t.Fatalf("damage to the protected player was not prevented: life %d, want %d", got, life)
	}
	// CR 611.2a: the effect outlives its now-exiled source.
	driveToStepAny(t, e, 2, 1, state.StepMain1)
	if kw := e.playerKeywords(0); !containsKW(kw, "Protection from everything") {
		t.Fatalf("the grant ended with its exiled source: playerKeywords(0)=%v", kw)
	}
	// The window ends at the cleanup before the controller's next turn
	// (CR 611.2b, gorge UntilTurn = next - 1): seat 0's turn 3 must begin
	// unprotected.
	driveToStepAny(t, e, 3, 0, state.StepMain1)
	if kw := e.playerKeywords(0); containsKW(kw, "Protection from everything") {
		t.Fatalf("the grant survived into the controller's next turn: %v", kw)
	}
}

func containsKW(kws []string, kw string) bool {
	for _, k := range kws {
		if k == kw {
			return true
		}
	}
	return false
}
