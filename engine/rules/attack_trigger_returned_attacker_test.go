// An Attacks trigger's Defined$ TriggeredAttackerLKICopy names the attacking
// creature that existed when the trigger fired. If that creature leaves the
// battlefield and returns before the trigger resolves, it is a new object
// (CR 400.7) and the trigger's "it gains ..." does not reach it.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const attackPumpEnchantment = "Name:Attack Pump Test\nManaCost:1 B\nTypes:Enchantment\n" +
	"T:Mode$ Attacks | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ TrigPump | TriggerDescription$ x\n" +
	"SVar:TrigPump:DB$ Pump | Defined$ TriggeredAttackerLKICopy | KW$ Deathtouch\n" +
	"Oracle:x\n"

// attackAndStackPump declares bear attacking and puts the enchantment's
// trigger on the stack, unresolved.
func attackAndStackPump(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	bear := onBoardReady(t, e, 0, "Name:Pump Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	onBoard(t, e, 0, attackPumpEnchantment)
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %d, want the one attack trigger", len(e.G.Stack))
	}
	if e.HasKeyword(bear, "Deathtouch") {
		t.Fatalf("precondition: bear has Deathtouch before the trigger resolved")
	}
	return e, bear
}

// TestAttackTriggerPumpsAttacker is the control: the attacker that stayed
// on the battlefield gains the keyword.
func TestAttackTriggerPumpsAttacker(t *testing.T) {
	t.Parallel()
	e, bear := attackAndStackPump(t)
	e.resolveTop()
	if !e.HasKeyword(bear, "Deathtouch") {
		t.Fatalf("attacker did not gain Deathtouch from its attack trigger")
	}
}

// TestAttackTriggerSkipsReturnedAttacker: the attacker died and returned
// while the trigger waited (Aclazotz returning as Temple of the Dead), so the
// pump finds no object.
func TestAttackTriggerSkipsReturnedAttacker(t *testing.T) {
	t.Parallel()
	e, bear := attackAndStackPump(t)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZGraveyard, To: state.ZBattlefield})
	if e.G.Obj(bear).Zone != state.ZBattlefield {
		t.Fatalf("precondition: bear zone = %s, want battlefield after the round trip", e.G.Obj(bear).Zone)
	}
	e.resolveTop()
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: the attack trigger did not resolve")
	}
	if e.HasKeyword(bear, "Deathtouch") {
		t.Fatalf("returned creature gained Deathtouch: the attack trigger reached a new object (CR 400.7)")
	}
}
