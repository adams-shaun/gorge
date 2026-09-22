package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// trig:DamageAll ("Whenever one or more <sources> deal damage to one or more
// <targets>") is the batch-level damage trigger Forge's
// GameAction.triggerDamageAll fires ONCE per damage batch if at least one
// matching source dealt damage to at least one matching target. Before this
// mode existed the trigger never fired at all (it was unregistered), so
// Contaminant Grafter's proliferate and the rest of the 8-card corpus family
// were dead.
//
// The batch face is pinned on the REAL corpus script: Contaminant Grafter's
// T:Mode$ DamageAll | CombatDamage$ True | ValidSource$ Creature.YouCtrl |
// ValidTarget$ Player | Execute$ TrigProliferate, with two attackers dealing
// combat damage to a player in ONE damage step. The mode must fire the
// proliferate ability exactly ONCE for the whole batch -- not once per Damage
// event -- which a counter carrier makes observable. The non-combat face is
// pinned on the same real card: its CombatDamage$ True requires the
// engine-side combatDamaging flag, so a creature's own ability dealing
// non-combat damage must not fire it.

// TestDamageAllFiresOncePerBatchOnRealCorpusScript is the brief's headline
// case. Two creatures you control (Contaminant Grafter itself and a Hill
// Giant) attack unblocked, so TWO Damage events land in the one combat damage
// batch; the "one or more" reading is ONE proliferate, which adds exactly one
// +1/+1 counter to a counter-bearing carrier. The old per-event reading would
// pose two proliferate asks and add two counters.
func TestDamageAllFiresOncePerBatchOnRealCorpusScript(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg,
		[]string{"Contaminant Grafter", "Hill Giant"},
		[]string{"Name:Counter Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"},
		nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	grafter := findBattlefield(t, e, 0, "Contaminant Grafter", 0)
	giant := findBattlefield(t, e, 0, "Hill Giant", 0)
	carrier := findBattlefield(t, e, 0, "Counter Bear", 0)

	// Precondition: the carrier really carries a counter (a proliferate with
	// no counter-bearing permanent is a no-op, so the assertion below would
	// pass vacuously without this).
	e.emit(events.Event{Kind: events.CounterChange, Obj: carrier, Counter: "P1P1", Amount: 2})
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 2 {
		t.Fatalf("precondition: carrier P1P1 = %d, want 2", got)
	}

	e.askAttackers()
	submitAttackers(t, e, grafter, giant)
	drainCombatDamagePriority(t, e)

	// Both attackers connected (the mode needs at least one matching source
	// AND one matching target in the batch, so a silent attack would make the
	// assertion vacuous).
	if got := e.G.Players[1].Life; got != 12 {
		t.Fatalf("precondition: defender life = %d, want 12 (5 + 3 combat damage landed)", got)
	}

	// Exactly ONE proliferate ask for the whole damage batch.
	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
		t.Fatalf("pending = %+v, want the single DamageAll proliferate ask", d)
	}
	carrierOpt := -1
	for i, o := range d.Options {
		if o.Obj == carrier {
			carrierOpt = i
		}
	}
	if carrierOpt < 0 {
		t.Fatalf("counter carrier not offered to proliferate: %+v", d.Options)
	}
	answerProliferate(t, e, d, carrierOpt)
	passUntilStackEmpty(t, e, 60)

	if got := e.G.Obj(carrier).Counter("P1P1"); got != 3 {
		t.Fatalf("carrier P1P1 = %d, want 3 (the batch fired the trigger ONCE: two attackers, one proliferate)", got)
	}
	if n := countCounterChanges(e, carrier, "P1P1", 1); n != 1 {
		t.Fatalf("logged %d CounterChange(P1P1, +1) on the carrier, want 1 (one batch instance)", n)
	}
	replayCheck(t, e, cfg)
}

// TestDamageAllNonCombatDamageDoesNotFire pins the CombatDamage$ half on the
// same real card, on the shape TestCombatDamageFlagDrivesTheCombatDamageGate
// uses: an identical Damage event fires Contaminant Grafter's DamageAll
// trigger in the combat state (the engine-side combatDamaging flag
// dealCombatDamage sets) and does NOT in the non-combat state. The combat
// leg is also a positive control -- it proves the mode is registered and
// queued, so this test cannot pass vacuously against an unregistered mode.
func TestDamageAllNonCombatDamageDoesNotFire(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg,
		[]string{"Contaminant Grafter"},
		[]string{"Name:Pinger\nManaCost:1 R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"},
		nil, nil)
	grafter := findBattlefield(t, e, 0, "Contaminant Grafter", 0)
	// A logged TurnChange/StepChange is the replay-consistent way to clear the
	// fixtures' summoning sickness and park the clock (the flag test's shape).
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	// Precondition: the trigger's carrier is really on the battlefield with
	// its DamageAll line, so a silent scan cannot make the legs vacuous.
	if f := e.G.Obj(grafter).Face(); f == nil || len(f.Triggers) == 0 || f.Triggers[0].Mode != "DamageAll" {
		t.Fatalf("precondition: grafter face = %+v, want a DamageAll trigger", f)
	}
	pinger := findBattlefield(t, e, 0, "Pinger", 0)

	// Combat leg: the flag set and e.damaging the pinger -- the state
	// dealCombatDamage runs every assignment's emit under. The pinger is a
	// creature you control, so ValidSource$ matches and the PLAYER recipient
	// matches ValidTarget$.
	e.damaging = pinger
	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 1})
	e.combatDamaging = false
	e.damaging = 0
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("combat-state Damage queued %d DamageAll triggers, want 1 (the positive control)", len(e.pendingTriggers))
	}
	e.pendingTriggers = nil

	// Non-combat leg: the identical event shape with the flag unset (a
	// creature's own ability dealing damage). It must not queue the
	// CombatDamage$ True trigger.
	e.damaging = pinger
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 1})
	e.damaging = 0
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("non-combat Damage queued %d DamageAll triggers, want 0 (CombatDamage$ True)", len(e.pendingTriggers))
	}
	replayCheck(t, e, cfg)
}
