package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLeylineTyrantXColorPayment pins the coloured X of a trigger's own cost:
// Leyline Tyrant's dies trigger is Cost$ X with XColor$ R, so the offered X is
// bounded by the RED sources, not the board's total mana, and the payment
// spends red pips. One Mountain beside three Forests offers X = 0..1; a
// generic reading (the old parseCost-only window) offered X = 0..4.
func TestLeylineTyrantXColorPayment(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Leyline Tyrant")
	id := searchMoveByName(t, e, "Leyline Tyrant", state.ZBattlefield)
	// the Forests come first on purpose: a generic cost's planner would tap
	// one of them, while the red pip can only spend the Mountain
	var forests []state.ObjID
	for i := 0; i < 3; i++ {
		forests = append(forests, xFoldLand(t, e, "Test Forest "+string(rune('A'+i)), "G"))
	}
	mountain := xFoldLand(t, e, "Test Mountain", "R")
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
	d := passUntilNonPriority(t, e, 40)
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "trigger_cost_x" {
		t.Fatalf("expected the choose-X announcement ask, got %+v", d)
	}
	if got := d.Options[len(d.Options)-1].Amount; got != 1 {
		t.Fatalf("largest X offered %d, want 1 (one red source; four mana on the board)", got)
	}
	submitChoices(t, e, xFoldAskIndex(t, d, 1))

	// the folded red pip is covered by activating the Mountain inside the
	// window; the Forests cannot pay it
	activateWindowMana(t, e)
	pay, _ := triggerCostWindowAsk(t, e)
	if pay < 0 {
		t.Fatalf("the folded red X cost was not offered as payable: %+v", e.Pending())
	}
	submitChoices(t, e, pay)
	if o := e.G.Obj(mountain); o == nil || !o.Tapped {
		t.Errorf("Mountain tapped = %v, want true (the red pip)", o != nil && o.Tapped)
	}
	for _, f := range forests {
		if o := e.G.Obj(f); o != nil && o.Tapped {
			t.Errorf("Forest tapped on a red X payment")
		}
	}
}
