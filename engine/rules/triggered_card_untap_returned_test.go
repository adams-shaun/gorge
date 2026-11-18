// A Taps trigger's Defined$ TriggeredCardLKICopy names the creature that became
// tapped when the trigger fired (Captain America, Living Legend's untap). If
// that creature changes zones before the trigger resolves, it is a new object
// (CR 400.7) and the untap does not reach it: The Mighty Thor, Jane Foster's
// return-tapped entry (CR 110.5) must stay tapped.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const tapsUntapTestEnchantment = "Name:Taps Untap Test\nManaCost:1 W\nTypes:Enchantment\n" +
	"T:Mode$ Taps | Execute$ UntapIt | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | TriggerDescription$ x\n" +
	"SVar:UntapIt:DB$ Untap | Defined$ TriggeredCardLKICopy\n" +
	"Oracle:x\n"

// tapsAndStackUntap puts the test enchantment and a creature on the
// battlefield, taps the creature and leaves the enchantment's untap trigger
// unresolved on the stack.
func tapsAndStackUntap(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	onBoard(t, e, 0, tapsUntapTestEnchantment)
	bear := onBoard(t, e, 0, "Name:Untap Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Tap, Obj: bear})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %d, want the one taps trigger", len(e.G.Stack))
	}
	if !e.G.Obj(bear).Tapped {
		t.Fatalf("precondition: bear is not tapped")
	}
	return e, bear
}

// TestTapsTriggerUntapsTappedCreature is the control: a creature that stayed on
// the battlefield is untapped.
func TestTapsTriggerUntapsTappedCreature(t *testing.T) {
	t.Parallel()
	e, bear := tapsAndStackUntap(t)
	e.resolveTop()
	if e.G.Obj(bear).Tapped {
		t.Fatalf("bear is still tapped after the untap trigger resolved")
	}
}

// TestTapsTriggerSkipsReturnedTappedCreature: the creature was exiled and
// returned tapped while the trigger waited, so the untap finds no object.
func TestTapsTriggerSkipsReturnedTappedCreature(t *testing.T) {
	t.Parallel()
	e, bear := tapsAndStackUntap(t)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZExile, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Tap, Obj: bear, Text: "entered tapped"})
	if o := e.G.Obj(bear); o.Zone != state.ZBattlefield || !o.Tapped {
		t.Fatalf("precondition: bear zone = %s tapped = %v, want a tapped battlefield entry", o.Zone, o.Tapped)
	}
	e.resolveTop()
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: the taps trigger did not resolve")
	}
	if !e.G.Obj(bear).Tapped {
		t.Fatalf("returned creature was untapped: the trigger reached a new object (CR 400.7)")
	}
}
