package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestExtortPipIgnoresRestrictedMana: Extort's {W/B} (CR 702.101a) is paid
// during a triggered ability's resolution, neither a spell cast nor an
// activation, so mana restricted to those (CR 106.6) cannot pay it. A seat
// holding only a restricted W declines (nothing charged, the batch intact);
// one holding a plain W beside it pays with the plain unit and leaves the
// restricted batch alone.
func TestExtortPipIgnoresRestrictedMana(t *testing.T) {
	bolt := card(t, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
	e := handEngineTokens(t, bolt)
	pl := &e.G.Players[0]
	pl.Pool = state.Mana{}
	pl.PersistentMana = state.Mana{}
	pl.Pool[state.MW] = 1
	pl.RestrictedMana = []state.ManaRestriction{{Color: "W", Amount: 1, Valid: "Spell.Creature"}}
	if pay.PayExtortPip(asPayer(e), 0) {
		t.Fatal("the restricted W paid Extort's pip")
	}
	if pl.Pool[state.MW] != 1 || len(pl.RestrictedMana) != 1 {
		t.Fatalf("a declined pip changed the pool: W %d, restricted %+v", pl.Pool[state.MW], pl.RestrictedMana)
	}

	// An unrestricted AddsNoCounter$ batch (empty Valid) pays anything.
	pl.RestrictedMana = []state.ManaRestriction{{Color: "W", Amount: 1, Valid: ""}}
	if !pay.PayExtortPip(asPayer(e), 0) || pl.Pool[state.MW] != 0 {
		t.Fatalf("an unrestricted batch did not pay: W %d", pl.Pool[state.MW])
	}

	pl.Pool[state.MW] = 2
	pl.RestrictedMana = []state.ManaRestriction{{Color: "W", Amount: 1, Valid: "Spell.Creature"}}
	if !pay.PayExtortPip(asPayer(e), 0) {
		t.Fatal("the plain W beside the restricted one did not pay")
	}
	if pl.Pool[state.MW] != 1 || len(pl.RestrictedMana) != 1 || pl.RestrictedMana[0].Amount != 1 {
		t.Fatalf("pay took the restricted unit: W %d, restricted %+v", pl.Pool[state.MW], pl.RestrictedMana)
	}
	if pay.PayExtortPip(asPayer(e), 0) {
		t.Fatal("a second pip was paid from the restricted unit alone")
	}
}
