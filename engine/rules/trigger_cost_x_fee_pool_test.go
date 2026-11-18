package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredCostXSupplyPricesAFeeSourceNetBesideFloatingMana: with mana
// floating, the payment-plan census lists a fee source (Viridescent Bog's
// "{1}, {T}: Add {B}{G}") at its gross production, so one floating mana plus the
// Bog read as 3 although the floating mana pays the Bog's fee and 2 mana
// remain. The announced X must be one the payment settles (CR 601.2b, 605.3b).
func TestTriggeredCostXSupplyPricesAFeeSourceNetBesideFloatingMana(t *testing.T) {
	t.Parallel()
	e, _, _ := xCensusBoard(t, xPaidLake)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 1})
	admits := func(state.ObjID, *cards.SA) bool { return true }
	w := paidSourceWindow{e.G, e.castWindowProbeAbilities, e.manaAbilityCC, e.castWindowAmount}
	raw := pay.PaymentPlanQueryUnits(asPayer(e), 0)
	pool := e.G.Players[0].Pool.Total()
	if pool != 1 || len(raw) != 1 {
		t.Fatalf("precondition: want 1 floating mana and the fee source in the census, got pool %d units %d", pool, len(raw))
	}
	if gross := triggeredCostXSupply(pool, raw, admits); gross != 3 {
		t.Fatalf("precondition: the raw census prices the Lake gross, supply %d, want 3", gross)
	}
	units := triggeredCostXFeeFree(w, 0, raw)
	supply := triggeredCostXSupply(pool, units, admits)
	supply += triggeredCostXPaidNet(w, 0, supply, units, admits)
	if supply != 2 {
		t.Fatalf("supply %d, want 2 (the floating mana pays the fee; two mana remain)", supply)
	}
}
