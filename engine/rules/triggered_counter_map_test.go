// PutCounter.TriggeredCounterMap$ (Captain Marvel, Apex Avenger; Bold
// Plagiarist; Aragorn, Company Leader): the put copies the per-kind counter
// map of the triggering CounterPlayerAddedAll batch instead of reading
// CounterType$/CounterNum$. The batch latch merges every CounterChange of one
// put action for a recipient into that map, exactly as Forge's
// GameEntityCounterTable carries one Multiset<CounterType> per cell.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// triggeredCounterMapGame seats the real Captain Marvel, Apex Avenger on seat
// 0's battlefield with the named counter recipient beside her. The trigger
// watches counter placement, not combat, so no turn or combat state is needed.
func triggeredCounterMapGame(t *testing.T, recipient string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	cm := onBoardCard(t, e, 0, mshCorpusCard(t, "Captain Marvel, Apex Avenger"))
	other := onBoard(t, e, 0, recipient)
	if e.G.Obj(cm).Zone != state.ZBattlefield || e.G.Obj(other).Zone != state.ZBattlefield {
		t.Fatal("precondition: Captain Marvel and the recipient must be on the battlefield")
	}
	trigs := e.G.Obj(cm).Face().Triggers
	if len(trigs) != 1 || trigs[0].Mode != "CounterPlayerAddedAll" {
		t.Fatalf("precondition: Captain Marvel triggers = %+v, want one CounterPlayerAddedAll", trigs)
	}
	if trigs[0].Effect == nil || trigs[0].Effect.Params["TriggeredCounterMap"] == "" {
		t.Fatal("precondition: Captain Marvel's trigger body does not carry TriggeredCounterMap$")
	}
	return e, cm, other
}

// putTwoKinds resolves a real api:PutCounter that places two +1/+1 counters
// and one flying counter on recipient. The engine's PutCounter emits one
// CounterChange per kind inside its own action bracket (effPutCounter's
// beginActionBatch), so this is the exact production path the trigger map has
// to merge.
func putTwoKinds(t *testing.T, e *Engine, recipient state.ObjID) {
	t.Helper()
	sa := card(t, "Name:Put Two Kinds\nTypes:Sorcery\n"+
		"A:SP$ PutCounter | Defined$ Targeted | CounterTypes$ P1P1,P1P1,Flying | CounterNum$ 1\nOracle:x\n").Faces[0].Abilities[0]
	prev := e.SetCounterAdder(0) // "you put" (ValidSource$ You)
	effects.Resolve(e, &effects.Ctx{Source: recipient, Controller: 0,
		Targets: []state.Target{{Obj: recipient}}}, sa)
	e.SetCounterAdder(prev)
}

func TestTriggeredCounterMapCopiesKindsAndCounts(t *testing.T) {
	t.Parallel()

	// Accepted: the trigger copies the whole map, not just the last event.
	t.Run("accepted", func(t *testing.T) {
		e, cm, other := triggeredCounterMapGame(t, "Name:Non-Kree Test Creature\nTypes:Creature Human\nPT:2/2\nOracle:x\n")
		before := len(e.pendingTriggers)
		putTwoKinds(t, e, other)
		if got := e.G.Obj(other).Counter("P1P1"); got != 2 {
			t.Fatalf("precondition: recipient +1/+1 counters = %d, want 2", got)
		}
		if got := e.G.Obj(other).Counter("Flying"); got != 1 {
			t.Fatalf("precondition: recipient flying counters = %d, want 1", got)
		}
		n := len(e.pendingTriggers) - before
		if n != 1 {
			t.Fatalf("one two-kind put action queued %d Captain Marvel triggers, want 1", n)
		}
		tc := e.pendingTriggers[before].Ctx.TriggerContext
		if len(tc.TriggerCounterMap) != 2 || tc.TriggerCounterMap[0].Kind != "P1P1" || tc.TriggerCounterMap[0].Count != 2 ||
			tc.TriggerCounterMap[1].Kind != "Flying" || tc.TriggerCounterMap[1].Count != 1 {
			t.Fatalf("trigger counter map = %+v, want [{P1P1 2} {Flying 1}]", tc.TriggerCounterMap)
		}
		if tc.TriggerAmount != 3 {
			t.Fatalf("trigger amount = %d, want the batch total 3", tc.TriggerAmount)
		}

		e.putTriggersOnStack()
		e.resolveTop()
		answerPutOptional(t, e, 0)
		passUntilStackEmpty(t, e, 30)

		if got := e.G.Obj(cm).Counter("P1P1"); got != 2 {
			t.Fatalf("Captain Marvel +1/+1 counters after the copy = %d, want 2", got)
		}
		if got := e.G.Obj(cm).Counter("Flying"); got != 1 {
			t.Fatalf("Captain Marvel flying counters after the copy = %d, want 1", got)
		}
	})

	// Declined: the same map is offered but places nothing.
	t.Run("declined", func(t *testing.T) {
		e, cm, other := triggeredCounterMapGame(t, "Name:Non-Kree Test Creature\nTypes:Creature Human\nPT:2/2\nOracle:x\n")
		putTwoKinds(t, e, other)
		e.putTriggersOnStack()
		e.resolveTop()
		answerPutOptional(t, e, 1)
		passUntilStackEmpty(t, e, 30)
		if got := e.G.Obj(cm).Counter("ALL"); got != 0 {
			t.Fatalf("declined copy left Captain Marvel with %d counters, want 0", got)
		}
	})

	// A Kree recipient fails Captain Marvel's intervening-if (X = 0, so
	// ConditionSVarCompare$ EQ0 is false) and never queues the trigger.
	t.Run("kree", func(t *testing.T) {
		e, _, other := triggeredCounterMapGame(t, "Name:Kree Test Creature\nTypes:Creature Kree Soldier\nPT:2/2\nOracle:x\n")
		before := len(e.pendingTriggers)
		putTwoKinds(t, e, other)
		if got := e.G.Obj(other).Counter("P1P1"); got != 2 {
			t.Fatalf("precondition: Kree recipient +1/+1 counters = %d, want 2", got)
		}
		if got := len(e.pendingTriggers) - before; got != 0 {
			t.Fatalf("a Kree recipient queued %d Captain Marvel triggers, want 0", got)
		}
	})
}
