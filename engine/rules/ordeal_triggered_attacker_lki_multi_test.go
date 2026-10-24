// cr-g101 follow-up: the ConditionDefined$ TriggeredAttackerLKICopy gate must
// bind the SINGLE triggering attacker, not the whole DeclareAttackers batch.
// A second attacker declared alongside the carrier must neither satisfy the
// Ordeals' counter gate nor Viv Vision's power gate.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// attackBatch declares ids attacking seat 1 in ONE DeclareAttackers event and
// drains every trigger it queues, including an Ordeal's sacrifice trigger.
func attackBatch(t *testing.T, e *Engine, ids ...state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: ids})
	for i := 0; i < 16; i++ {
		if len(e.pendingTriggers) == 0 && len(e.G.Stack) == 0 {
			return
		}
		e.putTriggersOnStack()
		if len(e.G.Stack) == 0 {
			return
		}
		e.resolveTop()
	}
	t.Fatal("attack triggers did not drain")
}

// TestOrdealConditionBindsOnlyItsEnchantedAttacker: another attacker in the
// same declaration carrying three +1/+1 counters must not satisfy the Ordeal's
// "if it has three or more +1/+1 counters" gate. The Ordeal sacrifices only
// once ITS enchanted creature reaches three.
func TestOrdealConditionBindsOnlyItsEnchantedAttacker(t *testing.T) {
	t.Parallel()
	e, bear, ordeal := newOrdealBoard(t)
	other := onBoardReady(t, e, 0, "Name:Other Attacker\nManaCost:3 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")

	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: other, Counter: "P1P1", Amount: 3})
	if got := e.G.Obj(bear).Counter("P1P1"); got != 1 {
		t.Fatalf("precondition: enchanted bear counters = %d, want 1 (below the gate)", got)
	}
	if got := e.G.Obj(other).Counter("P1P1"); got != 3 {
		t.Fatalf("precondition: other attacker counters = %d, want 3 (would satisfy the gate)", got)
	}

	attackBatch(t, e, bear, other)
	if e.G.Obj(ordeal).Zone != state.ZBattlefield {
		t.Fatalf("first attack: Ordeal left the battlefield at %d counters on the bear (the other attacker has 3), want it to stay", e.G.Obj(bear).Counter("P1P1"))
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("first attack: seat 0 life = %d, want 20 (no sacrifice payoff yet)", got)
	}

	// The enchanted creature itself now reaches three: the gate fires.
	attackBatch(t, e, bear, other)
	if got := e.G.Obj(bear).Counter("P1P1"); got != 3 {
		t.Fatalf("second attack: enchanted bear counters = %d, want 3", got)
	}
	if e.G.Obj(ordeal).Zone != state.ZGraveyard {
		t.Fatalf("second attack: Ordeal zone = %s, want graveyard (sacrificed at three counters)", e.G.Obj(ordeal).Zone)
	}
	if got := e.G.Players[0].Life; got != 30 {
		t.Fatalf("second attack: seat 0 life = %d, want 30 (Ordeal's sacrifice trigger gained 10)", got)
	}
}

// TestVivVisionConditionBindsOnlyViv: a power-4 creature attacking alongside
// the printed 2/2 Viv Vision must not satisfy HER power-4 gate; she draws only
// when her own power reaches 4.
func TestVivVisionConditionBindsOnlyViv(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	viv := onBoardCard(t, e, 0, corpusCard(t, "Viv Vision, Teen Synthezoid"))
	e.G.Obj(viv).SummonSick = false
	big := onBoardReady(t, e, 0, "Name:Big Attacker\nManaCost:4 G\nTypes:Creature Beast\nPT:4/4\nOracle:x\n")

	draw := func(ids ...state.ObjID) int {
		before := len(e.G.Zone(state.ZHand, 0))
		attackBatch(t, e, ids...)
		return len(e.G.Zone(state.ZHand, 0)) - before
	}

	if p := e.Derived(viv).Power; p != 2 {
		t.Fatalf("precondition: Viv Vision power = %d, want 2", p)
	}
	if p := e.Derived(big).Power; p != 4 {
		t.Fatalf("precondition: other attacker power = %d, want 4", p)
	}
	if got := draw(viv, big); got != 0 {
		t.Fatalf("power 2 Viv Vision drew %d card(s) alongside a power-4 attacker, want 0", got)
	}

	e.emit(events.Event{Kind: events.CounterChange, Obj: viv, Counter: "P1P1", Amount: 2})
	small := onBoardReady(t, e, 0, "Name:Small Attacker\nManaCost:2 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	if p := e.Derived(viv).Power; p != 4 {
		t.Fatalf("precondition: Viv Vision power = %d after two counters, want 4", p)
	}
	if got := draw(viv, small); got != 1 {
		t.Fatalf("power 4 Viv Vision drew %d card(s) alongside a power-2 attacker, want 1", got)
	}
}
