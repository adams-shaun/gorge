package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// activation_after_blockers_test.go pins ActivationAfterBlockers$ True:
// the offer exists during combat AFTER blockers are
// declared, and a priority window IN StepDeclareBlockers is by construction
// after that, because CR 509.1 makes declaring blockers a turn-based action
// at the START of the step and CR 509.4 grants priority only once it is done.
//
// engine/rules/turn.go's StepDeclareBlockers arm is the authority for that
// ordering: it calls e.askBlockers() unless declarationMadeThisStep(events.
// DeclareBlockers) already holds, and only then runs e.priorityRound(). No
// priority decision can exist in the step before the declaration.
//
// The old gate (`e.G.Step <= state.StepDeclareBlockers`) excluded the step
// itself, so the four xmage-port combat/BecomesBlockedTest scenarios (Trap
// Runner and friends) never got the window their paired
// `ActivationPhases$ Declare Blockers->EndCombat` range exists to provide.
// These tests pin the fix from both sides: the step is INCLUDED, and every
// step before it is still excluded.

// TestActivationAfterBlockersTrapRunnerOfferedOnceBlockersDeclared is the real
// corpus carrier: Trap Runner's `{T}: Target unblocked attacking creature
// becomes blocked. Activate only during combat after blockers are declared`
// (`ActivationPhases$ Declare Blockers->EndCombat | ActivationAfterBlockers$
// True`). The ability must be OFFERED at priority in the declare-blockers
// step once the blockers declaration is complete.
//
// Offer-only: Trap Runner's BecomesBlocked resolution is unrelated machinery
// this pin does not drive.
func TestActivationAfterBlockersTrapRunnerOfferedOnceBlockersDeclared(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	runner := searchCorpusCard(t, reg, "Trap Runner")
	e, cfg := activationDeck(t, 9314,
		[]*cards.Card{runner, card(t, actPhaseAllyBearSrc), mountainDeck(t, 1)[0], mountainDeck(t, 1)[0]},
		mountainDeck(t, 40))
	trapRunner := moveByName(t, e, 0, "Trap Runner", state.ZBattlefield)
	// Precondition: Trap Runner is really on the battlefield, and its printed
	// {T} ability carries BOTH the range and the rider this pin is about (a
	// corpus re-pin that dropped either would otherwise make the test vacuous).
	o := e.G.Obj(trapRunner)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Trap Runner is not on the battlefield: %+v", o)
	}
	sa := trapRunnerAbility(t, e, trapRunner)
	if got := sa.Params["ActivationPhases"]; got != "Declare Blockers->EndCombat" {
		t.Fatalf("Trap Runner's ability carries ActivationPhases$ %q, want \"Declare Blockers->EndCombat\"", got)
	}
	if got := sa.Params["ActivationAfterBlockers"]; got != "True" {
		t.Fatalf("Trap Runner's ability carries ActivationAfterBlockers$ %q, want \"True\"", got)
	}

	// Trap Runner stays HOME and untapped: its cost is {T}, so it could not
	// pay it in the very combat it was attacking in (declaring attackers taps
	// it). The attack that drives the step comes from a separate unblocked
	// attacker, which is also the ability's legal target.
	bear := moveByName(t, e, 0, "Ally Bear", state.ZBattlefield)
	fundPool(t, e, "")
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, bear)

	// AFTER blockers are declared: priority in the declare-blockers step.
	driveToBlockersPriority(t, e, 0)
	if e.G.Step != state.StepDeclareBlockers || e.G.Active != 0 {
		t.Fatalf("expected seat 0's declare-blockers step, got step %s active %d", e.G.Step, e.G.Active)
	}
	// Precondition: the declaration really happened before this priority
	// window -- without it the window would not be "after blockers are
	// declared" and the positive assertion would prove nothing.
	if !e.declarationMadeThisStep(events.DeclareBlockers) {
		t.Fatal("seat 0 holds declare-blockers priority with no DeclareBlockers event yet -- the CR 509.4 ordering turn.go guarantees is broken")
	}
	// Precondition: Trap Runner is untapped, so its {T} cost is payable and
	// the offer below is the gate's doing rather than the cost planner's.
	if o := e.G.Obj(trapRunner); o.Tapped {
		t.Fatal("Trap Runner is tapped -- its {T} cost is unpayable and the offer assertion would be vacuous")
	}
	if abilityFor(t, e, 0, trapRunner) == nil {
		t.Fatalf("Trap Runner's {T} ability was not offered at declare-blockers priority after blockers were declared: %+v", e.Pending())
	}
	// The compliance harness selects an activation by the ability's IR index
	// (oraclePickActivateByIndex: Option.Ability == ability_index), not by a
	// bare "an ability of this object" match, so assert that selector too --
	// a test that passed the loose check while the harness still failed would
	// prove nothing about the four BecomesBlockedTest scenarios.
	if _, ok := e.G.Obj(trapRunner).PileAbilityAt(0); !ok {
		t.Fatal("Trap Runner carries no pile ability at index 0 -- the harness's ability_index: 0 would select nothing")
	}
	found := false
	for _, op := range e.Pending().Options {
		if op.Obj == trapRunner && op.Ability == 0 && op.Kind == "ability" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Trap Runner's ability is not offered under ability_index 0 (what oraclePickActivateByIndex matches): %+v", e.Pending())
	}
	replayCheck(t, e, cfg)
}

// TestActivationAfterBlockersExcludesEarlierSteps is the negative half: an
// ability whose ActivationPhases$ range ALONE spans the whole of combat is
// still withheld in the declare-attackers step by ActivationAfterBlockers$
// alone, and offered in the declare-blockers step. Using a wide range makes
// the withhold provably the rider's doing, not the range's -- and it is the
// shape a `>=`-style over-correction would break.
func TestActivationAfterBlockersExcludesEarlierSteps(t *testing.T) {
	t.Parallel()
	const src = "Name:Wide Window Probe\nManaCost:1 W\nTypes:Creature Human Soldier\nPT:2/3\n" +
		"A:AB$ Pump | Cost$ T | ValidTgts$ Creature | ActivationPhases$ BeginCombat->EndCombat | ActivationAfterBlockers$ True | NumAtt$ +1 | SpellDescription$ probe\n" +
		"Oracle:x\n"
	e, cfg, id := newFixtureDeck(t, 9315, src, actPhaseAllyBearSrc)
	// The probe itself stays home untapped (its cost is {T}); a separate Bear
	// is the attacker that drives the declare-blockers step into existence.
	probe := moveByName(t, e, 0, "Wide Window Probe", state.ZBattlefield)
	if probe != id {
		t.Fatalf("moved probe %d but newFixtureDeck reported %d", probe, id)
	}
	bear := moveByName(t, e, 0, "Ally Bear", state.ZBattlefield)
	fundPool(t, e, "")

	// Precondition: the wide range accepts BOTH steps on its own, so neither
	// assertion below can be satisfied by the range gate alone.
	rangeSet, unknown := state.ParsePhases("BeginCombat->EndCombat")
	if len(unknown) != 0 || !rangeSet.Has(state.StepDeclareAttackers) || !rangeSet.Has(state.StepDeclareBlockers) {
		t.Fatalf("the wide range does not span both combat declaration steps: set=%v unknown=%v", rangeSet, unknown)
	}

	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, bear)
	if e.G.Step != state.StepDeclareAttackers {
		t.Fatalf("expected the declare-attackers step, got %s", e.G.Step)
	}
	// Precondition: the probed permanent is untapped, so its {T} cost is
	// payable and a withheld offer can only be the gate's doing.
	if o := e.G.Obj(id); o == nil || o.Tapped {
		t.Fatalf("the probe is tapped or gone (%+v) -- the offer assertions below would be vacuous", o)
	}
	if abilityFor(t, e, 0, id) != nil {
		t.Fatal("the probe ability was offered in the declare-attackers step -- ActivationAfterBlockers$ did not withhold it")
	}

	driveToBlockersPriority(t, e, 0)
	if e.G.Step != state.StepDeclareBlockers {
		t.Fatalf("expected the declare-blockers step, got %s", e.G.Step)
	}
	if !e.declarationMadeThisStep(events.DeclareBlockers) {
		t.Fatal("declare-blockers priority exists with no DeclareBlockers event -- precondition broken")
	}
	if e.G.Obj(id).Tapped {
		t.Fatal("the probe became tapped before the positive assertion -- it would be vacuous")
	}
	if abilityFor(t, e, 0, id) == nil {
		t.Fatalf("the probe ability was not offered once blockers were declared: %+v", e.Pending())
	}
	replayCheck(t, e, cfg)
}

// trapRunnerAbility returns the object's first printed AB$ (the {T} ability),
// failing loudly when the card carries none.
func trapRunnerAbility(t *testing.T, e *Engine, id state.ObjID) *cards.SA {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || len(o.Face().Abilities) == 0 {
		t.Fatalf("object %d carries no printed ability", id)
	}
	return o.Face().Abilities[0]
}
