package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The Pride of Hull Clade's {2}{U}{U} grants "Whenever this creature deals
// combat damage to a player, draw cards equal to its toughness" through two
// script copies of TrigDamage: the Animate grant on the target (the real
// trigger) and the Effect's Triggers$ copy, whose Card.Self is the Effect card
// and so never fires (CR 603.2). These tests pin one trigger on self-target and
// the unchanged other-target behaviour.

const prideBearSrc = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// prideActivateAndAttack activates the Pride's ability targeting target in
// seat 0's first main phase, attacks with target into the empty board, and
// drains combat damage and its triggers. It returns the number of
// trigger_order decisions seen and seat 0's draws from combat on.
func prideActivateAndAttack(t *testing.T, inline []string, targetName string) (e *Engine, cfg Config, attacker state.ObjID, orders, draws int) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg = combatTriggerBoard(t, reg, []string{"The Pride of Hull Clade"}, inline, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	pride := findBattlefield(t, e, 0, "The Pride of Hull Clade", 0)
	attacker = findBattlefield(t, e, 0, targetName, 0)
	if o := e.G.Obj(attacker); o.Tapped || o.SummonSick {
		t.Fatalf("precondition: %s tapped=%v sick=%v, want an untapped ready attacker", targetName, o.Tapped, o.SummonSick)
	}

	addMana(t, e, 0, "UUUU")
	opt := abilityOption(t, e, pride, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pride target decision %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == attacker {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no target option for %s: %+v", targetName, d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 30)

	before := countDraws(e, 0)
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	e.askAttackers()
	submitAttackers(t, e, attacker)
	drainCombatDamagePriority(t, e)
	for i := 0; i < 64 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KTriggerOrder {
			orders++
			choices := make([]int, len(d.Options))
			for j, o := range d.Options {
				choices[j] = o.Index
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatalf("answer trigger_order: %v", err)
			}
			continue
		}
		if len(e.G.Stack) == 0 {
			break
		}
		passUntilStackEmpty(t, e, 30)
	}
	return e, cfg, attacker, orders, countDraws(e, 0) - before
}

func TestPrideOfHullCladeSelfTargetDrawsOnce(t *testing.T) {
	t.Parallel()
	e, cfg, pride, orders, draws := prideActivateAndAttack(t, nil, "The Pride of Hull Clade")
	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("precondition: seat 1 life = %d, want 17 (the lone attacker, the Pride at 3 power, connected)", got)
	}
	if o := e.G.Obj(pride); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the Pride is in %s, want the battlefield", o.Zone)
	}
	if orders != 0 {
		t.Fatalf("saw %d trigger_order decision(s), want 0: one DamageDone trigger", orders)
	}
	if got := draws; got != 15 {
		t.Fatalf("seat 0 drew %d, want 15 (one trigger, toughness 15)", got)
	}
	replayCheck(t, e, cfg)
}

func TestPrideOfHullCladeOtherTargetDrawsToughness(t *testing.T) {
	t.Parallel()
	e, cfg, _, orders, draws := prideActivateAndAttack(t, []string{prideBearSrc}, "Bear")
	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("precondition: seat 1 life = %d, want 17 (the lone attacker, the Bear at 3 power, connected)", got)
	}
	if orders != 0 {
		t.Fatalf("saw %d trigger_order decision(s), want 0", orders)
	}
	if got := draws; got != 2 {
		t.Fatalf("seat 0 drew %d, want 2 (the Bear's toughness)", got)
	}
	replayCheck(t, e, cfg)
}
