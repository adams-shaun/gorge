package rules

// Rage Forger: "Whenever a creature you control with a +1/+1 counter on it
// attacks, you may have that creature deal 1 damage to target player or
// planeswalker." CR 603.5: the controller chooses at resolution whether an
// optional trigger's effect happens. The trigger line below is the corrected
// script (OptionalDecider$ You on the Attacks line, the fork fix); the
// engine reads optionality only from that parameter, so this pins that an
// Attacks trigger carrying it poses a KTriggerOptional, that "no" deals
// nothing and that "yes" deals 1 to the chosen target.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const rageForgerOptionalSrc = "Name:Rage Forger\nManaCost:2 R\nTypes:Creature Elemental Shaman\nPT:2/2\n" +
	"T:Mode$ Attacks | ValidCard$ Creature.YouCtrl+counters_GE1_P1P1 | TriggerZones$ Battlefield | OptionalDecider$ You | Execute$ TrigDamage | TriggerDescription$ x\n" +
	"SVar:TrigDamage:DB$ DealDamage | ValidTgts$ Player,Planeswalker | TgtPrompt$ x | NumDmg$ 1 | DamageSource$ TriggeredAttacker\n" +
	"Oracle:x\n"

// rageForgerAttack builds the board, declares the countered bear as an
// attacker at seat 1, answers the target ask with seat 1, and returns the
// optional election plus seat 1's life before the trigger resolves.
func rageForgerAttack(t *testing.T) (*Engine, *decision.Decision, int32) {
	t.Helper()
	e := layerEngine(t)
	forger := onBoard(t, e, 0, rageForgerOptionalSrc)
	bear := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 1})
	// PRECONDITION: the attacker matches ValidCard$ (a +1/+1 counter) and the
	// trigger source is a distinct permanent on the battlefield.
	if got := e.G.Obj(bear).Counter("P1P1"); got != 1 {
		t.Fatalf("precondition: bear P1P1 = %d, want 1", got)
	}
	if o := e.G.Obj(forger); o == nil || o.Zone != state.ZBattlefield || forger == bear {
		t.Fatalf("precondition: forger = %+v, want a distinct battlefield permanent", o)
	}
	life := e.G.Players[1].Life

	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	if got := len(e.pendingTriggers); got != 1 {
		t.Fatalf("Rage Forger queued %d triggers for one countered attacker, want 1", got)
	}
	e.putTriggersOnStack()
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no pending decision (stack %d)", len(e.G.Stack))
		}
		switch d.Kind {
		case decision.KTriggerOptional:
			if d.Player != 0 || len(d.Options) != 2 {
				t.Fatalf("optional election = %+v, want seat 0's yes/no", d)
			}
			return e, d, life
		case decision.KTarget:
			submitChoices(t, e, optionIdxForPlayer(t, d, 1))
		case decision.KPriority:
			if len(e.G.Stack) == 0 {
				t.Fatalf("the trigger resolved without posing KTriggerOptional (life %d -> %d)", life, e.G.Players[1].Life)
			}
			passOnce(t, e)
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
	}
	t.Fatal("KTriggerOptional never posed")
	return nil, nil, 0
}

// finishRageForger drives the remaining stack to empty and returns the
// damage seat 1 took.
func finishRageForger(t *testing.T, e *Engine, life int32) int32 {
	t.Helper()
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision after the election: %+v", d)
		}
		passOnce(t, e)
	}
	if n := len(e.G.Stack); n != 0 {
		t.Fatalf("stack still holds %d item(s)", n)
	}
	return life - e.G.Players[1].Life
}

func TestRageForgerAttackTriggerIsOptional(t *testing.T) {
	t.Parallel()
	t.Run("no", func(t *testing.T) {
		t.Parallel()
		e, d, life := rageForgerAttack(t)
		submitChoices(t, e, d.Options[1].Index)
		if dealt := finishRageForger(t, e, life); dealt != 0 {
			t.Fatalf("declining dealt %d damage, want 0", dealt)
		}
	})
	t.Run("yes", func(t *testing.T) {
		t.Parallel()
		e, d, life := rageForgerAttack(t)
		submitChoices(t, e, d.Options[0].Index)
		if dealt := finishRageForger(t, e, life); dealt != 1 {
			t.Fatalf("accepting dealt %d damage, want 1", dealt)
		}
	})
}
