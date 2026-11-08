// A ChangesZone trigger's Defined$ TriggeredCardLKICopy names the creature
// that entered when the trigger fired. If that creature leaves the
// battlefield and returns before the trigger resolves, it is a new object
// (CR 400.7) and the trigger's pump does not reach it. Unstoppable Slasher
// returning from its own dies trigger under Enduring Courage is the corpus
// case.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const entersPumpTestEnchantment = "Name:Enters Pump Test\nManaCost:1 R\nTypes:Enchantment\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other+YouCtrl | TriggerZones$ Battlefield | Execute$ TrigPump | TriggerDescription$ x\n" +
	"SVar:TrigPump:DB$ Pump | NumAtt$ +2 | KW$ Haste | Defined$ TriggeredCardLKICopy\n" +
	"Oracle:x\n"

// entersAndStackPump puts the test enchantment on the battlefield and enters a
// 2/2 creature, leaving the enchantment's trigger unresolved on the stack.
func entersAndStackPump(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	onBoard(t, e, 0, entersPumpTestEnchantment)
	bear := e.G.AddObject(card(t, "Name:Pump Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	bear.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), bear.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear.ID, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %d, want the one enters trigger", len(e.G.Stack))
	}
	if e.G.Obj(bear.ID).Zone != state.ZBattlefield {
		t.Fatalf("precondition: bear zone = %s, want battlefield", e.G.Obj(bear.ID).Zone)
	}
	if got := e.Power(bear.ID); got != 2 {
		t.Fatalf("precondition: bear power = %d, want its printed 2 before the pump resolves", got)
	}
	if e.HasKeyword(bear.ID, "Haste") {
		t.Fatalf("precondition: bear already has Haste before the trigger resolved")
	}
	return e, bear.ID
}

// TestEntersTriggerPumpsEnteringCreature is the control: a creature that stayed
// on the battlefield gets the pump.
func TestEntersTriggerPumpsEnteringCreature(t *testing.T) {
	t.Parallel()
	e, bear := entersAndStackPump(t)
	e.resolveTop()
	if got := e.Power(bear); got != 4 {
		t.Fatalf("bear power = %d, want 4 (2 printed + the resolved +2/+0)", got)
	}
	if !e.HasKeyword(bear, "Haste") {
		t.Fatalf("bear did not gain Haste from the resolved trigger")
	}
}

// TestEntersTriggerSkipsReturnedCreature: the creature left and returned while
// the trigger waited, so the pump finds no object.
func TestEntersTriggerSkipsReturnedCreature(t *testing.T) {
	t.Parallel()
	e, bear := entersAndStackPump(t)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZGraveyard, To: state.ZBattlefield})
	if e.G.Obj(bear).Zone != state.ZBattlefield {
		t.Fatalf("precondition: bear zone = %s, want battlefield after the round trip", e.G.Obj(bear).Zone)
	}
	e.resolveTop()
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: the enters trigger did not resolve")
	}
	if got := e.Power(bear); got != 2 {
		t.Fatalf("returned creature power = %d, want its printed 2: the trigger reached a new object (CR 400.7)", got)
	}
	if e.HasKeyword(bear, "Haste") {
		t.Fatalf("returned creature gained Haste: the trigger reached a new object (CR 400.7)")
	}
}
