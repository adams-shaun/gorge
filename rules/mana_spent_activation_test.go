package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Sunken Palace's real TriggersWhenSpent$ body is Mode$ SpellAbilityCast, so
// mana spent on an activated ability must queue its trigger just as it does
// for a spell. The payment is classified as an activation and its source
// attribution comes from the same restricted-mana batch used in production.
func TestSunkenPalaceManaSpentOnActivationQueuesTrigger(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cmdr := corpusCommander(t, reg, "Wort, Boggart Auntie")
	sunken := corpusCommander(t, reg, "Sunken Palace")
	ballista := corpusCommander(t, reg, "Walking Ballista")
	e, _ := colourIdentityGame(t, 921, FormatCommander, cmdr, nil, sunken, ballista)
	sunkenID := moveToBattlefieldByName(t, e, 0, "Sunken Palace")
	ballistaID := moveToBattlefieldByName(t, e, 0, "Walking Ballista")
	if o := e.G.Obj(sunkenID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Sunken Palace is not on the battlefield: %+v", o)
	}
	if o := e.G.Obj(ballistaID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("activated-ability source is not on the battlefield: %+v", o)
	}

	// Add the attributable mana batch exactly as ManaAdd's event encoding
	// does, then pay a U activation cost through the activation payment path.
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 1,
		Text: events.ManaRestrictionText("", sunkenID)})
	if ok, _, _, _, _ := e.payManaForSpent(0, ballistaID, true, ParseCost("U"), nil, pipRider{}); !ok {
		t.Fatal("activation mana payment failed")
	}
	if len(e.manaSpentSources) != 1 || e.manaSpentSources[0] != sunkenID {
		t.Fatalf("captured mana sources = %v, want Sunken Palace %d", e.manaSpentSources, sunkenID)
	}
	pa, ok := e.G.Obj(ballistaID).PileAbilityAt(0)
	if !ok || pa.SA == nil {
		t.Fatal("Walking Ballista has no first activated ability")
	}
	if pa.SA.API == "Mana" {
		t.Fatal("precondition: test activation unexpectedly is a mana ability")
	}
	push := events.Event{Kind: events.AbilityPush, Obj: ballistaID, Player: 0, Amount: 0}
	e.fireManaSpentTriggers(push, nil)
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pending TriggersWhenSpent triggers = %d, want one for SpellAbilityCast activation", len(e.pendingTriggers))
	}
	if e.pendingTriggers[0].Source != sunkenID {
		t.Fatalf("queued trigger source = %d, want Sunken Palace %d", e.pendingTriggers[0].Source, sunkenID)
	}
}
