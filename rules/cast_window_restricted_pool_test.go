package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// Restricted units count toward Pool.Total but cannot certify a generic
// activation cost: the payment window must not promise a source it cannot pay.
func TestCastWindowGenericCostFailsClosedOnRestrictedPool(t *testing.T) {
	b := equipWindowGame(t, 519, equipWindowGenericSrc)
	e := b.engine
	addMana(t, e, 0, "CCCCCC")
	pl := e.G.Players[0]
	pl.RestrictedMana = append(pl.RestrictedMana, state.ManaRestriction{Color: "C", Amount: 6, Valid: "Spell"})
	if pl.Pool.Total() != 6 || len(pl.RestrictedMana) != 1 {
		t.Fatalf("precondition: pool=%d restrictions=%+v", pl.Pool.Total(), pl.RestrictedMana)
	}
	if got, want := e.Power(b.brute), int32(4); got != want {
		t.Fatalf("brute power=%d want %d", got, want)
	}

	opt, ok := findAbilityOption(e, b.belt, 0)
	if !ok {
		t.Fatal("equip not offered")
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("target ask=%+v", d)
	}
	pc := e.cast
	if pc == nil {
		t.Fatal("missing pending cast")
	}
	for _, unit := range e.castWindowUnits(pc) {
		if unit.id == b.byName["GenericRock"] {
			t.Fatalf("generic-cost source promised despite restricted-only activation mana: units=%+v", unit)
		}
	}
}
