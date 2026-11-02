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

// TestCastingProhibitionReadsTheFaceDownSpell: CR 708.4 has "effects that care
// about the characteristics of a spell" see only the face-down set, so a
// prohibition judges the face-down cast by a nameless colourless 2/2 creature
// with mana value 0, and the printed cast by the card.
func TestCastingProhibitionReadsTheFaceDownSpell(t *testing.T) {
	t.Parallel()
	const blueMorph = "Name:Blue Morph\nManaCost:4 U\nTypes:Creature Bird\nPT:3/3\nK:Morph:U\nOracle:x\n"
	const landMorph = "Name:Cave Morph\nTypes:Land\nK:Morph:2\nOracle:x\n"
	for _, tc := range []struct {
		name, valid, hand string
		printed, faceDown bool
	}{
		{"a colour", "Creature.Blue", blueMorph, false, true},
		{"a name", "Card.namedBlue Morph", blueMorph, false, true},
		{"a low mana value", "Card.cmcLE3", blueMorph, true, false},
		{"a land that casts as a creature", "Creature", landMorph, false, false},
		{"nothing it matches", "Instant", blueMorph, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, tc.hand))
			id := e.G.Zone(state.ZHand, 0)[0]
			battlePerm(t, e, 1, "Name:Prohibitor\nTypes:Enchantment\nS:Mode$ CantBeCast | ValidCard$ "+tc.valid+" | Caster$ Player\nOracle:x\n")
			addMana(t, e, 0, "UUUUU")
			e.priorityRound()
			var printed, faceDown bool
			for _, o := range e.Pending().Options {
				if o.Obj == id && o.Kind == "cast" {
					if o.Mode == "morphed" {
						faceDown = true
					} else {
						printed = true
					}
				}
			}
			if printed != tc.printed || faceDown != tc.faceDown {
				t.Fatalf("printed offered %v, face-down %v; want %v, %v", printed, faceDown, tc.printed, tc.faceDown)
			}
		})
	}
}
