package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	testRecalledLesson = "Name:Test Recalled Lesson\nManaCost:2 R\nTypes:Instant Lesson\n" +
		"A:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ Recall one life.\n" +
		"K:Flashback:6 R\nOracle:x\n"
	testOldLesson = "Name:Test Old Lesson\nManaCost:R\nTypes:Sorcery Lesson\nOracle:x\n"
)

// CR 601.2a moves the card to the stack before CR 601.2f determines the total
// cost, so a flashback spell does not count itself in its own graveyard. The
// keeper's reduction needs three Lessons in every spelling: the spell and two
// others are one short, and the offer and the charge both read seven mana.
func TestCR601_2fFlashbackSpellDoesNotCountItself(t *testing.T) {
	t.Parallel()
	const head = "Name:Test Lesson Keeper\nManaCost:U\nTypes:Creature Human\nPT:1/2\n"
	const desc = " | Description$ Studied Lessons make your spells cheaper.\n"
	for _, tc := range []struct {
		name, keeper string
		others       int // Lessons in the graveyard besides the spell
		charge       int32
	}{
		{"IsPresent$ one short", head +
			"S:Mode$ ReduceCost | ValidCard$ Card.nonCreature | Type$ Spell | Activator$ You | Amount$ 1 | IsPresent$ Lesson.YouOwn | PresentZone$ Graveyard | PresentCompare$ GE3" + desc, 2, 7},
		{"IsPresent$ reached by others", head +
			"S:Mode$ ReduceCost | ValidCard$ Card.nonCreature | Type$ Spell | Activator$ You | Amount$ 1 | IsPresent$ Lesson.YouOwn | PresentZone$ Graveyard | PresentCompare$ GE3" + desc, 3, 6},
		{"CheckSVar$ one short", head +
			"S:Mode$ ReduceCost | ValidCard$ Card.nonCreature | Type$ Spell | Activator$ You | Amount$ 1 | CheckSVar$ TestStudied | SVarCompare$ GE3" + desc +
			"SVar:TestStudied:Count$ValidGraveyard Lesson.YouOwn\n", 2, 7},
		{"Amount$ counts the others only", head +
			"S:Mode$ ReduceCost | ValidCard$ Card.nonCreature | Type$ Spell | Activator$ You | Amount$ TestStudied" + desc +
			"SVar:TestStudied:Count$ValidGraveyard Lesson.YouOwn\n", 2, 5},
		{"Delirium one type short", head +
			"S:Mode$ ReduceCost | ValidCard$ Card.nonCreature | Type$ Spell | Activator$ You | Amount$ 1 | Condition$ Delirium" + desc, 3, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, testRecalledLesson))
			costtailPermCard(t, e, 0, card(t, tc.keeper))
			spell := e.G.Zone(state.ZHand, 0)[0]
			e.emit(events.Event{Kind: events.MoveZone, Obj: spell, From: state.ZHand, To: state.ZGraveyard})
			for i := 0; i < tc.others; i++ {
				other := testOldLesson
				if tc.name == "Delirium one type short" {
					// Sorcery, artifact and land: the spell's instant would be the fourth type.
					other = []string{testOldLesson, "Name:Test Trinket\nTypes:Artifact\nOracle:x\n", "Name:Test Field\nTypes:Land\nOracle:x\n"}[i]
				}
				o := e.G.AddObject(card(t, other), 0)
				e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZGraveyard})
			}
			pool := &e.G.Players[0].Pool
			pool[state.MR], pool[state.MC] = 1, tc.charge-2
			if hasCastOption(e.legalActions(0), spell) {
				t.Fatalf("flashback offered a mana short of %d", tc.charge)
			}
			pool[state.MC] = 6
			if !hasCastOption(e.legalActions(0), spell) {
				t.Fatal("flashback not offered on seven mana")
			}

			castMode(t, e, spell, "flashback")
			costtailDrive(t, e, nil)

			if paid := 7 - pool.Total(); e.G.Obj(spell).Zone != state.ZStack || paid != tc.charge {
				t.Fatalf("zone %s, paid %d, want the stack and %d", e.G.Obj(spell).Zone, paid, tc.charge)
			}
		})
	}
}

// A hand cast is priced the same way: the spell is not a card in hand when its
// cost is determined, so "no sorceries in hand" holds for the last one.
func TestCR601_2fHandSpellDoesNotCountItself(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:Test Last Word\nManaCost:2 R\nTypes:Sorcery\n"+
		"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ 2 | EffectZone$ All | IsPresent$ Sorcery.YouOwn | PresentZone$ Hand | PresentCompare$ EQ0 | Description$ Cheaper as your last sorcery in hand.\n"+
		"A:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ Gain one life.\nOracle:x\n"))
	spell := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MR] = 1
	if !hasCastOption(e.legalActions(0), spell) {
		t.Fatal("not offered on {R}: the spell counted itself in hand")
	}
}
