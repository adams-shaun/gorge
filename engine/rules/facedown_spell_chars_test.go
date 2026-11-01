package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestFaceDownSpellHasOnlyFaceDownCharacteristics: CR 708.4, "effects that care
// about the characteristics of a spell will see only the face-down spell's
// characteristics". A morph cast face down is a 2/2 creature spell with no name,
// subtypes or mana cost, so casting Birchlore Rangers that way is no Elf spell for
// Lys Alana Huntmaster's trigger, and its derived characteristics are the
// face-down ones while it is on the stack.
func TestFaceDownSpellHasOnlyFaceDownCharacteristics(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Birchlore Rangers", "Lys Alana Huntmaster")
	searchMoveByName(t, e, "Lys Alana Huntmaster", state.ZBattlefield)
	rangers := searchMoveByName(t, e, "Birchlore Rangers", state.ZHand)
	addMana(t, e, 0, "GGG")
	casts := e.Pending().Options
	picked := false
	for _, o := range casts {
		if o.Obj == rangers && o.Mode == "morphed" {
			submitChoices(t, e, o.Index)
			picked = true
			break
		}
	}
	if !picked {
		t.Fatalf("no face-down cast of Birchlore Rangers: %+v", casts)
	}
	o := e.G.Obj(rangers)
	if o.Zone != state.ZStack || !o.FaceDown {
		t.Fatalf("face-down spell: zone %v, face down %v", o.Zone, o.FaceDown)
	}
	d := e.Derived(rangers)
	if d.Power != 2 || d.Toughness != 2 {
		t.Errorf("derived P/T on the stack %d/%d, want 2/2", d.Power, d.Toughness)
	}
	for _, ty := range d.Types {
		if ty != "Creature" {
			t.Errorf("derived types on the stack %v, want only Creature", d.Types)
		}
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush {
			t.Errorf("a trigger fired on the face-down cast: %+v", ev)
		}
	}
	if len(e.pendingTriggers) != 0 {
		t.Errorf("%d pending trigger(s) from the face-down cast", len(e.pendingTriggers))
	}
}
