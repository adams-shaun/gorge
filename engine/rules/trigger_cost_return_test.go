package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// celebrantWindow casts Stockpiling Celebrant over the permanents built from
// sources and lets its enters trigger reach the cost's pay/decline window.
func celebrantWindow(t *testing.T, sources ...string) (*Engine, []state.ObjID) {
	t.Helper()
	e := handEngine(t, corpusAlternativeCard(t, "Stockpiling Celebrant"))
	var ids []state.ObjID
	for _, src := range sources {
		ids = append(ids, onBoard(t, e, 0, src))
	}
	e.G.Players[0].Pool[state.MW], e.G.Players[0].Pool[state.MC] = 1, 2
	e.priorityRound()
	submitChoices(t, e, castModeOption(t, e, e.G.Zone(state.ZHand, 0)[0], ""))
	for i := 0; i < 10; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision before the cost window")
		}
		if d.Kind != decision.KPriority {
			return e, ids
		}
		passOnceP(t, e)
	}
	t.Fatalf("the cost window never opened: %+v", e.Pending())
	return nil, nil
}

// windowOffers reports whether the pending cost window offers the pay arm.
func windowOffers(e *Engine) bool {
	for _, o := range e.Pending().Options {
		if o.Kind == "trigger_cost_pay" {
			return true
		}
	}
	return false
}

// TestTriggerCostReturnsPermanentToHand pins a triggered ability's Return cost
// as payable (CR 118.1), at the window and through the settle. It is offered
// only when another nonland permanent can pay, it moves the chosen one to its
// owner's hand, and only then does the body run.
func TestTriggerCostReturnsPermanentToHand(t *testing.T) {
	t.Parallel()
	const (
		gear = "Name:Gear\nTypes:Artifact\nOracle:x\n"
		land = "Name:Bare Land\nTypes:Land\nOracle:x\n"
	)
	t.Run("a land cannot pay", func(t *testing.T) {
		t.Parallel()
		e, _ := celebrantWindow(t, land)
		if windowOffers(e) {
			t.Fatal("pay offered with nothing to return")
		}
	})
	t.Run("the only candidate returns unasked", func(t *testing.T) {
		t.Parallel()
		e, ids := celebrantWindow(t, gear)
		submitChoices(t, e, optionForKind(t, e, "trigger_cost_pay"))
		if z := e.G.Obj(ids[0]).Zone; z != state.ZHand {
			t.Fatalf("the Gear is in zone %v, want hand", z)
		}
		if d := e.Pending(); d == nil || d.Kind != decision.KArrange {
			t.Fatalf("the body's scry did not follow the payment: %+v", d)
		}
	})
	t.Run("the payer chooses among candidates", func(t *testing.T) {
		t.Parallel()
		e, ids := celebrantWindow(t, gear, gear)
		submitChoices(t, e, optionForKind(t, e, "trigger_cost_pay"))
		submitChoices(t, e, optionForObj(e.Pending(), ids[1]))
		if a, b := e.G.Obj(ids[0]).Zone, e.G.Obj(ids[1]).Zone; a != state.ZBattlefield || b != state.ZHand {
			t.Fatalf("zones after choosing the second Gear: %v and %v", a, b)
		}
	})
	t.Run("declining returns nothing and skips the body", func(t *testing.T) {
		t.Parallel()
		e, ids := celebrantWindow(t, gear)
		submitChoices(t, e, optionForKind(t, e, "trigger_cost_decline"))
		if z := e.G.Obj(ids[0]).Zone; z != state.ZBattlefield {
			t.Fatalf("the Gear is in zone %v after declining", z)
		}
		if d := e.Pending(); d != nil && d.Kind == decision.KArrange {
			t.Fatal("the body ran after declining")
		}
	})
}

// optionForKind finds the pending decision's first option of kind.
func optionForKind(t *testing.T, e *Engine, kind string) int {
	t.Helper()
	for _, o := range e.Pending().Options {
		if o.Kind == kind {
			return o.Index
		}
	}
	t.Fatalf("no %s option: %+v", kind, e.Pending().Options)
	return -1
}
