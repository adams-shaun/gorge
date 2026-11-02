package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// sharedPumpScript gives every creature its controller controls the listed
// keywords a creature they control has, until end of turn.
const sharedPumpScript = "Name:Pooled Effort\nManaCost:2 W\nTypes:Sorcery\n" +
	"A:SP$ PumpAll | ValidCards$ Creature.YouCtrl | KW$ Flying & Landwalk & Protection & Vigilance | " +
	"SharedKeywordsZone$ Battlefield | SharedRestrictions$ Creature.YouCtrl\nOracle:x\n"

// fixtureInZone adds a card outside the seat's deck to one of its zones.
func fixtureInZone(t *testing.T, e *Engine, p state.PlayerID, z state.Zone, src string) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, src), p)
	o.Zone = z
	e.G.SetZone(z, p, append(slices.Clone(e.G.Zone(z, p)), o.ID))
	return o.ID
}

// TestPumpAllGrantsOnlyTheSharedKeywords pins Forge's PumpAllEffect reading of
// SharedKeywordsZone$ (read): only a keyword some matching card has is granted,
// and a family word grants the spelling that card has.
func TestPumpAllGrantsOnlyTheSharedKeywords(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 11, sharedPumpScript)
	flyer := fixtureInZone(t, e, 0, state.ZBattlefield, "Name:Winged\nManaCost:1 W\nTypes:Creature Bird\nPT:1/1\nK:Flying\nOracle:x\n")
	walker := fixtureInZone(t, e, 0, state.ZBattlefield, "Name:Walker\nManaCost:1 G\nTypes:Creature Elf\nPT:1/1\nK:Landwalk:Forest\nOracle:x\n")
	warded := fixtureInZone(t, e, 0, state.ZBattlefield, "Name:Warded\nManaCost:1 W\nTypes:Creature Cleric\nPT:1/1\nK:Protection from red\nOracle:x\n")
	bystander := fixtureInZone(t, e, 1, state.ZBattlefield, "Name:Rival\nManaCost:1 W\nTypes:Creature Knight\nPT:1/1\nK:Vigilance\nOracle:x\n")
	sa := card(t, sharedPumpScript).Faces[0].SpellAbility()
	effects.Resolve(e, &effects.Ctx{Controller: 0, Source: flyer}, sa)
	for _, id := range []state.ObjID{flyer, walker, warded} {
		kws := e.Derived(id).Keywords
		for _, want := range []string{"Flying", "Landwalk:Forest", "Protection from red"} {
			if !slices.Contains(kws, want) {
				t.Errorf("%s has %v, want %s", e.G.Obj(id).Face().Name, kws, want)
			}
		}
		if slices.Contains(kws, "Vigilance") {
			t.Errorf("%s gained vigilance, which only an opponent's creature has: %v", e.G.Obj(id).Face().Name, kws)
		}
	}
	if kws := e.Derived(bystander).Keywords; slices.Contains(kws, "Flying") {
		t.Errorf("the opponent's creature was pumped: %v", kws)
	}
}

// TestPumpGrantsOnlyTheSharedKeywords is the same reading on a single-object
// Pump, the corpus's "this creature has each keyword of the cards exiled with
// it" spelling.
func TestPumpGrantsOnlyTheSharedKeywords(t *testing.T) {
	t.Parallel()
	script := "Name:Shared Self\nManaCost:2 W\nTypes:Creature Beast\nPT:2/2\n" +
		"A:AB$ Pump | Cost$ 1 | Defined$ Self | KW$ Reach & Menace | SharedKeywordsZone$ Graveyard | SharedRestrictions$ Creature.YouOwn\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 12, script)
	self := fixtureInZone(t, e, 0, state.ZBattlefield, script)
	fixtureInZone(t, e, 0, state.ZGraveyard, "Name:Fallen\nManaCost:1 W\nTypes:Creature Spider\nPT:1/1\nK:Reach\nOracle:x\n")
	sa := card(t, script).Faces[0].Abilities[0]
	effects.Resolve(e, &effects.Ctx{Controller: 0, Source: self}, sa)
	kws := e.Derived(self).Keywords
	if !slices.Contains(kws, "Reach") || slices.Contains(kws, "Menace") {
		t.Fatalf("Pump granted %v, want Reach and not Menace", kws)
	}
}

// TestConditionOpponentTurn pins Forge's SpellAbilityCondition reading of
// ConditionOpponentTurn$ (read): the gated ability runs only while an opponent
// of its controller is the active player, whichever seat it is resolving for.
func TestConditionOpponentTurn(t *testing.T) {
	t.Parallel()
	script := "Name:Gated Bolt\nManaCost:R\nTypes:Sorcery\n" +
		"A:SP$ DealDamage | Defined$ You | NumDmg$ 2 | ConditionOpponentTurn$ True\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 13, script)
	sa := card(t, script).Faces[0].SpellAbility()
	active := e.G.Active
	before := e.G.Players[active].Life
	effects.Resolve(e, &effects.Ctx{Controller: active}, sa)
	if got := e.G.Players[active].Life; got != before {
		t.Fatalf("the active player's own gated ability dealt damage: life %d, want %d", got, before)
	}
	other := 1 - active
	before = e.G.Players[other].Life
	effects.Resolve(e, &effects.Ctx{Controller: other}, sa)
	if got := e.G.Players[other].Life; got != before-2 {
		t.Fatalf("the non-active player's gated ability: life %d, want %d", got, before-2)
	}
}
