// A compound Defined$ that includes the Triggered-card family must take the
// CR 400.7 move strip for its whole resolved set: "Self & TriggeredCardLKICopy"
// names the moved object through the family member even when the other member
// (Self, the resolving enchantment) is not itself the moved object.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const compoundEntersPumpTestEnchantment = "Name:Compound Enters Pump Test\nManaCost:1 R\nTypes:Enchantment\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other+YouCtrl | TriggerZones$ Battlefield | Execute$ Boosts | TriggerDescription$ x\n" +
	"SVar:Boosts:DB$ Pump | NumAtt$ +1 | Defined$ Self & TriggeredCardLKICopy\n" +
	"Oracle:x\n"

// compoundEntersAndStackPump puts the test enchantment on the battlefield and
// enters a 2/2 creature, leaving the enchantment's trigger unresolved.
func compoundEntersAndStackPump(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	onBoard(t, e, 0, compoundEntersPumpTestEnchantment)
	bear := e.G.AddObject(card(t, "Name:Compound Pump Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	bear.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), bear.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear.ID, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %d, want the one enters trigger", len(e.G.Stack))
	}
	if got := e.Power(bear.ID); got != 2 {
		t.Fatalf("precondition: bear power = %d, want its printed 2 before the pump resolves", got)
	}
	return e, bear.ID
}

// TestCompoundEntersTriggerPumpsEnteringCreature is the control: the creature
// stayed, so the compound selector's family member pumps it once.
func TestCompoundEntersTriggerPumpsEnteringCreature(t *testing.T) {
	t.Parallel()
	e, bear := compoundEntersAndStackPump(t)
	e.resolveTop()
	if got := e.Power(bear); got != 3 {
		t.Fatalf("bear power = %d, want 3 (2 printed + the resolved +1/+0)", got)
	}
}

// TestCompoundEntersTriggerSkipsReturnedCreature: the creature left and
// returned while the trigger waited, so the family member finds no object.
func TestCompoundEntersTriggerSkipsReturnedCreature(t *testing.T) {
	t.Parallel()
	e, bear := compoundEntersAndStackPump(t)
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
		t.Fatalf("returned creature power = %d, want its printed 2: the compound trigger reached a new object (CR 400.7)", got)
	}
}
