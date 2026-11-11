package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// damageThisTurnToOpps makes one player-targeted Damage event land (the same
// emitted-event shape engine/rules/host_turn_ledger.go folds) and asserts the
// per-turn ledger saw it, so every gate below reads a real non-zero ledger
// entry rather than a fixture that never moved the value the rule reads.
func damageThisTurnToOpps(t *testing.T, e *Engine, p state.PlayerID, amount int32) {
	t.Helper()
	before := e.G.Players[p].Life
	e.emit(events.Event{Kind: events.Damage, Player: p, Amount: amount})
	if got := e.DamageTakenThisTurn(p); got < amount {
		t.Fatalf("precondition: DamageTakenThisTurn(%d) = %d after %d damage", p, got, amount)
	}
	if got := e.G.Players[p].Life; got != before-amount {
		t.Fatalf("precondition: seat %d life = %d after %d damage, want %d", p, got, amount, before-amount)
	}
}

// resolveEndStepTrigger puts whatever the reached end step queued onto the
// stack, asserts the top is the named card's trigger, and resolves it. The
// step's trigger may already be pushed by the priority round that entered the
// step, so the assertion is on the stack, not on the pending queue.
func resolveEndStepTrigger(t *testing.T, e *Engine, source state.ObjID) {
	t.Helper()
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("end step stack = %v, want exactly the one trigger", e.G.Stack)
	}
	top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1])
	if top == nil || top.Source != source {
		t.Fatalf("top of stack = %+v, want the trigger sourced at %d", top, source)
	}
	e.resolveTop()
}

// TestAntagonismDamageToOppsThisTurn drives the real corpus Antagonism through
// the end-step gate its script carries: AntagonismX is a
// TriggeredPlayer$DamageToOppsThisTurn body compared with LE0 by
// ConditionSVarCompare.
//
// The trigger fires at every player's end step; it deals 2 to that player only
// when they have NO opponent dealt damage this turn. Before the evaluator arm
// existed the body answered (0, false), so CheckSVarHolds failed OPEN and the
// gate read a fake zero -- every end step dealt 2.
func TestAntagonismDamageToOppsThisTurn(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	antagonism, ok := reg.Lookup("Antagonism")
	if !ok {
		t.Fatal("corpus fixture: Antagonism is missing")
	}
	if missing := reg.Unsupported(antagonism, effects.Supported()); len(missing) != 0 {
		t.Fatalf("Antagonism is unsupported: %v", missing)
	}

	// (b) No damage to seat 0's opponent: the gate holds, so seat 0 loses 2.
	t.Run("no opponent damage deals 2", func(t *testing.T) {
		e, cfg := phaseRecurEngine(t, []string{"Antagonism"}, nil)
		id := moveByName(t, e, 0, "Antagonism", state.ZBattlefield)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Antagonism is not on the battlefield: %+v", o)
		}
		if got := e.DamageTakenThisTurn(1); got != 0 {
			t.Fatalf("precondition: seat 1 already took %d damage", got)
		}
		passUntilStep(t, e, 1, state.StepEnd)
		resolveEndStepTrigger(t, e, id)
		if got := e.G.Players[0].Life; got != 18 {
			t.Fatalf("seat 0's life after its end step = %d, want 18 (no opponent damage -> Antagonism deals 2)", got)
		}
		replayCheck(t, e, cfg)
	})

	// (c) Seat 1 was dealt damage: the gate fails, so seat 0 loses nothing.
	t.Run("opponent damage suppresses it", func(t *testing.T) {
		e, cfg := phaseRecurEngine(t, []string{"Antagonism"}, nil)
		id := moveByName(t, e, 0, "Antagonism", state.ZBattlefield)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Antagonism is not on the battlefield: %+v", o)
		}
		damageThisTurnToOpps(t, e, 1, 1)
		passUntilStep(t, e, 1, state.StepEnd)
		resolveEndStepTrigger(t, e, id)
		if got := e.G.Players[0].Life; got != 20 {
			t.Fatalf("seat 0's life after its end step = %d, want 20 (seat 1 was dealt damage)", got)
		}
		replayCheck(t, e, cfg)
	})

	// (d1) The same gate from seat 1's perspective: damage to seat 0 earlier
	// this turn suppresses seat 1's damage.
	t.Run("opponent damage suppresses it on the other end step", func(t *testing.T) {
		e, cfg := phaseRecurEngine(t, []string{"Antagonism"}, nil)
		id := moveByName(t, e, 0, "Antagonism", state.ZBattlefield)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Antagonism is not on the battlefield: %+v", o)
		}
		// Drive to seat 1's own turn first: the per-turn ledger resets at
		// TurnChange, so the damage must be dealt on the turn whose end step
		// reads it.
		passUntilStep(t, e, 2, state.StepMain1)
		damageThisTurnToOpps(t, e, 0, 1)
		passUntilStep(t, e, 2, state.StepEnd)
		resolveEndStepTrigger(t, e, id)
		if got := e.G.Players[1].Life; got != 20 {
			t.Fatalf("seat 1's life after its end step = %d, want 20 (seat 0 was dealt damage)", got)
		}
		replayCheck(t, e, cfg)
	})

	// (d2) Damage to SEAT 1 ITSELF is not "one of their opponents": the gate
	// still holds and seat 1 takes exactly 2 more.
	t.Run("own damage does not satisfy the unless", func(t *testing.T) {
		e, cfg := phaseRecurEngine(t, []string{"Antagonism"}, nil)
		id := moveByName(t, e, 0, "Antagonism", state.ZBattlefield)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Antagonism is not on the battlefield: %+v", o)
		}
		// Drive to seat 1's own turn first: the per-turn ledger resets at
		// TurnChange, so the damage must land on the turn whose end step
		// reads it.
		passUntilStep(t, e, 2, state.StepMain1)
		damageThisTurnToOpps(t, e, 1, 1)
		before := e.G.Players[1].Life // 19
		passUntilStep(t, e, 2, state.StepEnd)
		resolveEndStepTrigger(t, e, id)
		if got := before - e.G.Players[1].Life; got != 2 {
			t.Fatalf("seat 1 took %d from its own end step, want 2 (its own damage is not an opponent's)", got)
		}
		replayCheck(t, e, cfg)
	})
}

// TestBloodcrazedGoblinDamageToOppsAttackGate drives the real corpus
// Bloodcrazed Goblin's CantAttack static: its X SVar is a
// PlayerCountPropertyYou$DamageToOppsThisTurn body, held against LT1.
//
// The goblin is blocked while no opponent was dealt damage this turn, and the
// gate releases it the moment one was -- both through the same attackBlocked
// read the offer list and validator use.
func TestBloodcrazedGoblinDamageToOppsAttackGate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	goblin, ok := reg.Lookup("Bloodcrazed Goblin")
	if !ok {
		t.Fatal("corpus fixture: Bloodcrazed Goblin is missing")
	}
	if missing := reg.Unsupported(goblin, effects.Supported()); len(missing) != 0 {
		t.Fatalf("Bloodcrazed Goblin is unsupported: %v", missing)
	}

	e, cfg := phaseRecurEngine(t, []string{"Bloodcrazed Goblin"}, nil)
	id := moveByName(t, e, 0, "Bloodcrazed Goblin", state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Bloodcrazed Goblin is not on the battlefield: %+v", o)
	}
	if got := e.DamageTakenThisTurn(1); got != 0 {
		t.Fatalf("precondition: seat 1 already took %d damage", got)
	}
	if !e.attackBlocked(id, 1, 0) {
		t.Fatal("Bloodcrazed Goblin may attack although no opponent was dealt damage this turn")
	}
	damageThisTurnToOpps(t, e, 1, 1)
	if e.attackBlocked(id, 1, 0) {
		t.Fatal("Bloodcrazed Goblin is still blocked although an opponent was dealt damage this turn")
	}
	replayCheck(t, e, cfg)
}
