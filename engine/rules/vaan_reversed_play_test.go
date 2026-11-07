package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestVaanStreetThiefReversedPlayCreatesTheTreasure: the play is ANSWERED,
// but the exiled Bears' {1}{G} cast cannot be paid (no mana is added), so the
// answer settle begins the cast and reverses it (CR 733.1-2). The card goes
// back to exile, it was never played, so ForgetPlayed$ must leave it in the
// remembered set and the chained Treasure gate fires — exactly one Treasure.
// Before the playBegan guard, ctx.Play still named the reversed card and the
// forget dropped it, so no Treasure was created.
func TestVaanStreetThiefReversedPlayCreatesTheTreasure(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg, vaanID, _ := vaanEngine(t, reg)
	bearID, _ := vaanPlayAsk(t, e, vaanID)
	// No ManaAdd: the offered cast's {1}{G} is unpayable, so it reverses
	// inside the settlement instead of committing.
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 60)

	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("the reversed card is %v, want back in exile (CR 733.1)", o)
	}
	if n := countTokensNamedOnSeat(t, e, 0, "Treasure Token"); n != 1 {
		t.Fatalf("%d Treasure token(s) after the REVERSED play, want exactly 1 (the card was never played)", n)
	}
	if o := e.G.Obj(vaanID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Vaan left the battlefield: %v", o)
	}
	replayCheck(t, e, cfg)
}
