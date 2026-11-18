package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const xPaidForest = "Name:Paid Forest\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ G\nOracle:x\n"

const xPaidLake = "Name:Paid Lake\nTypes:Land\nA:AB$ Mana | Cost$ 1 T | Produced$ U R | SpellDescription$ Add {U}{R}.\nOracle:x\n"

// TestTriggeredCostXOfferCountsAFeeSourceNet: a mana ability with a literal fee and a
// bigger production (Ferrous Lake) is outside the payment-window census, but the
// announcement still counts what it adds beyond its fee (CR 605.3b). Two lands and the
// Lake fund X = 0..3: two taps, then the Lake's fee out of one of them for two mana.
func TestTriggeredCostXOfferCountsAFeeSourceNet(t *testing.T) {
	t.Parallel()
	e, _, _ := xCensusBoard(t, xPaidForest, xPaidForest, xPaidLake)
	entrant := e.G.AddObject(card(t, "Name:Big Entrant\nManaCost:X G\nTypes:Creature Hydra\nPT:0/0\nOracle:x\n"), 0)
	entrant.Zone = state.ZLibrary
	ids := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	e.G.SetZone(state.ZLibrary, 0, append(ids, entrant.ID))
	entrant.X = 2
	e.emit(events.Event{Kind: events.MoveZone, Obj: entrant.ID, From: state.ZLibrary, To: state.ZBattlefield})
	e.pending = nil
	e.priorityRound()
	d := passUntilNonPriority(t, e, 80)
	if d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "trigger_cost_x" {
		t.Fatalf("expected the choose-X announcement ask, got %+v", d)
	}
	if n := len(d.Options); n != 4 {
		got := make([]int, 0, n)
		for _, o := range d.Options {
			got = append(got, o.Amount)
		}
		t.Fatalf("X options %v, want 0..3", got)
	}
}

// TestTriggeredCostXPaidNetNeedsTheFeeFunded: the fee comes out of mana the other
// sources supply, so a lone fee source with nothing to pay it adds nothing.
func TestTriggeredCostXPaidNetNeedsTheFeeFunded(t *testing.T) {
	t.Parallel()
	e, _, _ := xCensusBoard(t, xPaidLake)
	admits := func(state.ObjID, *cards.SA) bool { return true }
	units := pay.PaymentPlanQueryUnits(asPayer(e), 0)
	if got := triggeredCostXPaidNet(engineWindow{e}, 0, 0, units, admits); got != 0 {
		t.Fatalf("a fee source with no supply added %d, want 0", got)
	}
	if got := triggeredCostXPaidNet(engineWindow{e}, 0, 1, units, admits); got != 1 {
		t.Fatalf("a fee source beside 1 supply added %d, want 1 (two mana for a one-mana fee)", got)
	}
}
