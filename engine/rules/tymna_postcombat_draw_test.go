package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Tymna the Weaver's postcombat trigger: "you may pay X life, X = opponents
// dealt combat damage this turn. If you do, draw X." The SVar:X head
// (PlayerCountRegisteredOpponents$HasPropertywasDealtCombatDamageThisTurn) is
// a FIXED fold: no announcement ask, then the ordinary pay/decline election.

type tymnaRun struct {
	e                  *Engine
	pay, decline       int
	life, draws, hand0 int
}

// tymnaWindow puts Tymna on seat 0's battlefield, optionally lands one combat
// hit on seat 1 first, drives to the postcombat main and returns the open
// trigger-cost election.
func tymnaWindow(t *testing.T, hit bool) tymnaRun {
	t.Helper()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Tymna the Weaver")
	id := searchMoveByName(t, e, "Tymna the Weaver", state.ZBattlefield)
	if hit {
		e.combatHitsThisTurn = append(e.combatHitsThisTurn, e.combatHit(1, id, 2))
	}
	if got := len(e.CombatDamageToPlayersThisTurn()); (got == 1) != hit {
		t.Fatalf("precondition: combat hits recorded = %d, hit = %v", got, hit)
	}
	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepMain2)
	d := passUntilNonPriority(t, e, 40)
	if d.Kind != decision.KChoose {
		t.Fatalf("expected the trigger-cost election, got %+v", d)
	}
	r := tymnaRun{e: e, pay: -1, decline: -1, life: int(e.G.Players[0].Life)}
	for _, o := range d.Options {
		switch o.Kind {
		case "trigger_cost_x":
			t.Fatalf("a FIXED X shape posed a choose-X announcement ask: %+v", d.Options)
		case "trigger_cost_pay":
			r.pay = o.Index
		case "trigger_cost_decline":
			r.decline = o.Index
		}
	}
	if r.decline < 0 {
		t.Fatalf("no decline option: %+v", d.Options)
	}
	return r
}

func (r *tymnaRun) answer(t *testing.T, idx int) (life, draws int) {
	t.Helper()
	mark := len(r.e.L.Events)
	submitChoices(t, r.e, idx)
	passUntilStackEmpty(t, r.e, 20)
	for _, ev := range r.e.L.Events[mark:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			draws++
		}
	}
	return int(r.e.G.Players[0].Life), draws
}

func TestTymnaPostcombatPayLifeDrawsDamageCount(t *testing.T) {
	t.Parallel()
	t.Run("one hit pays 1 draws 1", func(t *testing.T) {
		r := tymnaWindow(t, true)
		if r.pay < 0 {
			t.Fatalf("PayLife<1> was not offered as payable: %+v", r.e.Pending())
		}
		life, draws := r.answer(t, r.pay)
		if life != r.life-1 || draws != 1 {
			t.Fatalf("paid: life %d (want %d), draws %d (want 1)", life, r.life-1, draws)
		}
	})
	t.Run("one hit decline", func(t *testing.T) {
		r := tymnaWindow(t, true)
		life, draws := r.answer(t, r.decline)
		if life != r.life || draws != 0 {
			t.Fatalf("declined: life %d (want %d), draws %d (want 0)", life, r.life, draws)
		}
	})
	t.Run("no hit", func(t *testing.T) {
		r := tymnaWindow(t, false)
		t.Logf("X=0 election: pay=%d decline=%d", r.pay, r.decline)
		if r.pay < 0 {
			t.Fatalf("gorge poses decline-only at X = 0")
		}
		life, draws := r.answer(t, r.pay)
		if life != r.life || draws != 0 {
			t.Fatalf("X=0 pay: life %d (want %d), draws %d (want 0)", life, r.life, draws)
		}
	})
}
