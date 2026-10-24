// cr-g101: ConditionDefined$ TriggeredAttackerLKICopy must resolve to the
// attacking creature an Attacks trigger captured, so the Theros Ordeals
// sacrifice themselves at three +1/+1 counters and Viv Vision draws with
// power 4. Both drive the real corpus cards' own trigger chains.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ordealAttack declares bear attacking seat 1 (a direct DeclareAttackers
// emit, so no combat decision is involved) and drains every trigger it
// queues, including the Ordeal's own sacrifice trigger.
func ordealAttack(t *testing.T, e *Engine, bear state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
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
	t.Fatal("Ordeal triggers did not drain")
}

// newOrdealBoard places a ready bear and a real Ordeal of Heliod attached to
// it, both controlled by seat 0.
func newOrdealBoard(t *testing.T) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	bear := onBoardReady(t, e, 0, "Name:Ordeal Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	ordeal := onBoardCard(t, e, 0, corpusCard(t, "Ordeal of Heliod"))
	e.emit(events.Event{Kind: events.Attach, Obj: ordeal, IDs: []state.ObjID{bear}})
	if got := e.G.Obj(ordeal).AttachedTo; got != bear {
		t.Fatalf("precondition: Ordeal AttachedTo = %d, want bear %d", got, bear)
	}
	if !hasTypeWord(e.G.Obj(ordeal).Face().Types, "Aura") {
		t.Fatalf("precondition: Ordeal is not parsed as an Aura")
	}
	return e, bear, ordeal
}

// TestOrdealSacrificesAtThreeCounters: the first two attacks put a +1/+1
// counter on the enchanted creature and leave the Ordeal on the battlefield;
// the third reaches three counters and sacrifices it, firing the Ordeal's
// own sacrifice trigger (10 life).
func TestOrdealSacrificesAtThreeCounters(t *testing.T) {
	t.Parallel()
	e, bear, ordeal := newOrdealBoard(t)

	for attack := 1; attack <= 2; attack++ {
		ordealAttack(t, e, bear)
		if got := e.G.Obj(bear).Counter("P1P1"); got != int32(attack) {
			t.Fatalf("after attack %d: bear counters = %d, want %d", attack, got, attack)
		}
		if e.G.Obj(ordeal).Zone != state.ZBattlefield {
			t.Fatalf("after attack %d: Ordeal left the battlefield at %d counters, want it to stay", attack, attack)
		}
		if got := e.G.Players[0].Life; got != 20 {
			t.Fatalf("after attack %d: seat 0 life = %d, want 20 (no sacrifice payoff yet)", attack, got)
		}
	}

	ordealAttack(t, e, bear)
	if got := e.G.Obj(bear).Counter("P1P1"); got != 3 {
		t.Fatalf("after third attack: bear counters = %d, want 3", got)
	}
	if e.G.Obj(ordeal).Zone != state.ZGraveyard {
		t.Fatalf("after third attack: Ordeal zone = %s, want graveyard (sacrificed)", e.G.Obj(ordeal).Zone)
	}
	if got := e.G.Players[0].Life; got != 30 {
		t.Fatalf("after third attack: seat 0 life = %d, want 30 (Ordeal's sacrifice trigger gained 10)", got)
	}
}

// TestVivVisionDrawsWithPower4Attacker: Viv Vision at her printed 2/2 does
// not draw when she attacks; with two +1/+1 counters (power 4) she does.
func TestVivVisionDrawsWithPower4Attacker(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	viv := onBoardCard(t, e, 0, corpusCard(t, "Viv Vision, Teen Synthezoid"))
	e.G.Obj(viv).SummonSick = false

	draw := func() int {
		before := len(e.G.Zone(state.ZHand, 0))
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{viv}})
		e.putTriggersOnStack()
		if len(e.G.Stack) == 0 {
			t.Fatalf("Viv Vision's attack trigger was not placed on the stack")
		}
		e.resolveTop()
		return len(e.G.Zone(state.ZHand, 0)) - before
	}

	if p := e.Derived(viv).Power; p != 2 {
		t.Fatalf("precondition: Viv Vision power = %d, want 2", p)
	}
	if got := draw(); got != 0 {
		t.Fatalf("power 2 Viv Vision drew %d card(s), want 0", got)
	}

	e.emit(events.Event{Kind: events.CounterChange, Obj: viv, Counter: "P1P1", Amount: 2})
	if p := e.Derived(viv).Power; p != 4 {
		t.Fatalf("precondition: Viv Vision power = %d after two counters, want 4", p)
	}
	if got := draw(); got != 1 {
		t.Fatalf("power 4 Viv Vision drew %d card(s), want 1", got)
	}
}
