package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestApheliaEffectDamageDoneOnceOncePerBatch pins CR 603.2c/510.2 for an
// Effect's Triggers$ line: Aphelia's {4}{B} Effect registers a Mode$
// DamageDoneOnce delayed trigger, and two Gorgons dealing combat damage to
// one player in the same combat damage step are ONE triggering condition.
// The delayed-trigger walk must latch the damage batch exactly as the
// printed and granted walks do (trigger_delayed.go checkEventDelayedTriggers),
// so the player loses half their life once, not once per Damage event.
func TestApheliaEffectDamageDoneOnceOncePerBatch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg,
		[]string{"Aphelia, Viper Whisperer", "Masked Gorgon"},
		nil, nil, nil)
	aphelia := findBattlefield(t, e, 0, "Aphelia, Viper Whisperer", 0)
	scourge := findBattlefield(t, e, 0, "Masked Gorgon", 0)
	if aphelia == scourge {
		t.Fatalf("precondition: both Gorgons on the battlefield under one id %d", aphelia)
	}

	// Activate {4}{B}: the Effect registers the DamageDoneOnce delayed
	// trigger.
	startLife := e.G.Players[1].Life
	if startLife != 20 {
		t.Fatalf("precondition: defender life = %d, want 20", startLife)
	}
	// A logged TurnChange clears the attackers' summoning sickness exactly as
	// the board helper's direct field clear does, so replay rebuilds it.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	for _, r := range "BBBBB" {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(r), Amount: 1})
	}
	activateAbilityOn(t, e, aphelia)
	passUntilStackEmpty(t, e, 20)
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].EventMode != "DamageDoneOnce" {
		t.Fatalf("precondition: delayed registrations = %+v, want one DamageDoneOnce", e.G.Delayed)
	}

	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	e.askAttackers()
	submitAttackersOnly(t, e, aphelia, scourge)
	// Cross the declare-attackers priority window, declining Aphelia's
	// optional attack-trigger payment, and stop at the first combat-damage
	// priority decision -- the stacked combat-damage triggers are there.
	for i := 0; i < 64; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while crossing to combat damage")
		}
		if e.G.Step == state.StepCombatDamage && d.Kind == decision.KPriority {
			break
		}
		switch d.Kind {
		case decision.KPriority:
			submitPass(t, e)
		case decision.KTriggerOrder:
			idxs := make([]int, 0, d.Min)
			for _, o := range d.Options[:d.Min] {
				idxs = append(idxs, o.Index)
			}
			submitChoices(t, e, idxs...)
		case decision.KChoose:
			declined := false
			for _, o := range d.Options {
				if o.Kind == "trigger_cost_decline" {
					submitChoices(t, e, o.Index)
					declined = true
					break
				}
			}
			if declined {
				continue
			}
			// The optional attack trigger's {1}{B/G} hybrid pip: pay it (the
			// Snake token it makes enters after attackers are declared).
			paid := false
			for _, o := range d.Options {
				if o.Kind == "pay_B" || o.Kind == "pay_G" {
					submitChoices(t, e, o.Index)
					paid = true
					break
				}
			}
			if !paid {
				t.Fatalf("unexpected KChoose crossing to combat damage: %+v", d)
			}
		default:
			t.Fatalf("unexpected decision crossing to combat damage: %+v", d)
		}
	}
	if e.G.Step != state.StepCombatDamage {
		t.Fatalf("never reached the combat damage step (step %s)", e.G.Step)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("Aphelia's delayed trigger: stack depth = %d, want 1 (one trigger per damage batch, not per Damage event)", len(e.G.Stack))
	}
	if src := e.G.Obj(e.G.Stack[0]).Source; src != aphelia {
		t.Fatalf("stacked trigger source = %d, want Aphelia %d", src, aphelia)
	}

	before := e.G.Players[1].Life
	if before != 14 {
		t.Fatalf("precondition: defender life after combat damage = %d, want 14", before)
	}
	passUntilStackEmpty(t, e, 20)
	// The damage took 1 (Aphelia) + 5 (Scourge) = 6 life, then the one
	// halving loses half of what is left, rounded up: 14 -> 7.
	if got := e.G.Players[1].Life; got != 7 {
		t.Fatalf("defender life = %d, want 7 (one half-life loss, not one per damage event)", got)
	}
	replayCheck(t, e, cfg)
}
