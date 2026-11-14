package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	cfdBlueMorpher = "Name:Test Blue Morpher\nManaCost:4 U\nTypes:Creature Dragon\nPT:3/3\nK:Megamorph:2 U\nOracle:x\n"
	cfdBlueReducer = "Name:Test Blue Reducer\nManaCost:1 W\nTypes:Creature Wall\nPT:0/3\n" +
		"S:Mode$ ReduceCost | ValidCard$ Card.Blue,Card.Green | Type$ Spell | Activator$ You | Amount$ 1 | Description$ Green spells and blue spells you cast cost {1} less to cast.\nOracle:x\n"
)

// CR 708.4 and CR 601.2f: a face-down spell is a colourless creature spell, so
// "blue spells you cast cost {1} less" does not reach the {3} face-down cast of
// a blue card (the printed cast of the same card is reduced).
func TestFaceDownCastOfBlueCardIsNotReducedByBlueDiscount(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	spell := cfHand(e, cfdBlueMorpher, t, 0)
	o := e.G.AddObject(card(t, cfdBlueReducer), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})

	cfPool(e, "UCCC") // the printed {4}{U} costs {3}{U} under the discount
	if !cfModes(e, spell)[""] {
		t.Fatalf("precondition: the printed blue cast is not discounted: %v", cfModes(e, spell))
	}
	cfPool(e, "CC")
	if cfModes(e, spell)["megamorphed"] {
		t.Fatalf("the face-down cast was offered from {2}: the blue discount reduced a colourless spell")
	}
	cfPool(e, "CCC")
	if !cfModes(e, spell)["megamorphed"] {
		t.Fatalf("precondition: the face-down cast is not offered from {3}: %v", cfModes(e, spell))
	}
	cfCast(t, e, spell, "megamorphed")
	if left := e.G.Players[0].Pool.Total(); left != 0 {
		t.Fatalf("pool after the face-down cast = %d, want 0 (charged {3})", left)
	}
}
