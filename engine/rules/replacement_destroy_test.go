package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// An R:Event$ Destroy line at each destruction choke point. The medic spells
// the line of "if another creature you control with a +1/+1 counter on it would
// be destroyed, instead remove all +1/+1 counters from it and all damage already
// dealt to it is healed".
const destroyLineMedicSrc = "Name:Surly Medic\nManaCost:2 W W\nTypes:Legendary Creature Human Doctor\nPT:2/2\n" +
	"R:Event$ Destroy | ActiveZones$ Battlefield | ValidCard$ Creature.Other+YouCtrl+counters_GE1_P1P1 | ReplaceWith$ DBRemove | Description$ x\n" +
	"SVar:DBRemove:DB$ RemoveCounter | Defined$ ReplacedCard | CounterType$ P1P1 | CounterNum$ All | SubAbility$ DBHeal\n" +
	"SVar:DBHeal:DB$ HealDamage | Defined$ ReplacedCard\n" +
	"Oracle:x\n"

const destroyLineBearSrc = "Name:Line Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

func TestDestroyLineSparesFromDestroyEffects(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	murder := mustCorpusCard(t, reg, "Murder")
	wrath := mustCorpusCard(t, reg, "Wrath of God")
	for _, c := range []struct {
		name  string
		spell string
	}{{"effDestroy", "Murder"}, {"effDestroyAll", "Wrath of God"}} {
		t.Run(c.name, func(t *testing.T) {
			e, cfg := tokenReplGame(t, 7, murder, wrath)
			spell := moveSeededCard(t, e, 0, mustCorpusCard(t, reg, c.spell), state.ZHand)
			putToken(t, e, 0, destroyLineMedicSrc, state.ZBattlefield)
			bear := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
			e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 2})
			e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 1})
			plain := putToken(t, e, 1, destroyLineBearSrc, state.ZBattlefield)
			addMana(t, e, 0, "WWWWBBB")

			castDestroyAt(t, e, spell, bear)

			b := e.G.Obj(bear)
			if b.Zone != state.ZBattlefield || b.Counter("P1P1") != 0 || b.Damage != 0 {
				t.Fatalf("bear zone %s, %d counters, %d damage; want spared with none of either", b.Zone, b.Counter("P1P1"), b.Damage)
			}
			if c.spell == "Wrath of God" && e.G.Obj(plain).Zone == state.ZBattlefield {
				t.Fatal("the opposing bear has no replacement and should be destroyed")
			}
			replayCheck(t, e, cfg)
		})
	}
}

func TestDestroyLineSparesFromLethalDamage(t *testing.T) {
	e, _ := tokenReplGame(t, 13)
	putToken(t, e, 0, destroyLineMedicSrc, state.ZBattlefield)
	bear := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 1})
	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "Deathtouched", Amount: 1})
	e.checkStateBased()
	b := e.G.Obj(bear)
	if b.Zone != state.ZBattlefield || b.Counter("P1P1") != 0 || b.Damage != 0 || b.Counter("Deathtouched") != 0 {
		t.Fatalf("bear zone %s, %d counters, %d damage, %d deathtouch marks; want spared and clean",
			b.Zone, b.Counter("P1P1"), b.Damage, b.Counter("Deathtouched"))
	}
	// With its counters gone the line no longer names it.
	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 2})
	e.checkStateBased()
	if z := e.G.Obj(bear).Zone; z == state.ZBattlefield {
		t.Fatal("a second lethal hit should destroy the bear")
	}
}

// TestRegenerationLinesRegenerateUnlessTheDestructionForbids: Knight of the Holy
// Nimbus regenerates with no shield (CR 701.19a), and Wrath of God's "can't be
// regenerated" takes the line out of the event.
func TestRegenerationLinesRegenerateUnlessTheDestructionForbids(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	knight := mustCorpusCard(t, reg, "Knight of the Holy Nimbus")
	murder := mustCorpusCard(t, reg, "Murder")
	wrath := mustCorpusCard(t, reg, "Wrath of God")

	e, cfg := tokenReplGame(t, 7, knight, murder)
	spell := moveSeededCard(t, e, 0, murder, state.ZHand)
	id := moveSeededCard(t, e, 0, knight, state.ZBattlefield)
	e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 1})
	addMana(t, e, 0, "BBB")
	castDestroyAt(t, e, spell, id)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || !o.Tapped || o.Damage != 0 {
		t.Fatalf("Knight zone %s, tapped %v, damage %d; want regenerated", o.Zone, o.Tapped, o.Damage)
	}
	replayCheck(t, e, cfg)

	e, _ = tokenReplGame(t, 17, knight, wrath)
	spell = moveSeededCard(t, e, 0, wrath, state.ZHand)
	id = moveSeededCard(t, e, 0, knight, state.ZBattlefield)
	addMana(t, e, 0, "WWWW")
	castDestroyAt(t, e, spell, id)
	if z := e.G.Obj(id).Zone; z == state.ZBattlefield {
		t.Fatal("Wrath of God's destruction can't be regenerated from")
	}
}

// TestMossbridgeTrollRegeneratesFromLethalDamage is the lethal-damage choke point
// on the same line, and the regeneration heals what would have re-killed it.
func TestMossbridgeTrollRegeneratesFromLethalDamage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	troll := mustCorpusCard(t, reg, "Mossbridge Troll")
	e, _ := tokenReplGame(t, 13, troll)
	id := moveSeededCard(t, e, 0, troll, state.ZBattlefield)
	e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 5})
	e.checkStateBased()
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || !o.Tapped || o.Damage != 0 {
		t.Fatalf("Troll zone %s, tapped %v, damage %d; want regenerated", o.Zone, o.Tapped, o.Damage)
	}
}

// TestCracklingEmergenceSacrificesItselfForTheLand: the sacrifice unattaches the
// Aura first, and "that land" is still the land it enchanted (CR 608.2h).
func TestCracklingEmergenceSacrificesItselfForTheLand(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	aura := mustCorpusCard(t, reg, "Crackling Emergence")
	mountain := mustCorpusCard(t, reg, "Mountain")
	murder := mustCorpusCard(t, reg, "Murder")
	e, cfg := tokenReplGame(t, 7, aura, mountain, murder)
	spell := moveSeededCard(t, e, 0, murder, state.ZHand)
	land := moveSeededCard(t, e, 0, mountain, state.ZBattlefield)
	auraID := attachCorpusAura(t, e, 0, aura, land)
	addMana(t, e, 0, "BBB")

	castDestroyAt(t, e, spell, land)

	if z := e.G.Obj(land).Zone; z != state.ZBattlefield {
		t.Fatalf("land zone %s, want battlefield", z)
	}
	if z := e.G.Obj(auraID).Zone; z != state.ZGraveyard {
		t.Fatalf("Crackling Emergence zone %s, want graveyard", z)
	}
	if !e.HasKeyword(land, "Indestructible") {
		t.Fatal("the land should have gained indestructible until end of turn")
	}
	replayCheck(t, e, cfg)
}

// TestPyramidsShieldsTheNextDestruction: an Effect's Destroy line applies once
// and heals its land, then the Effect is gone.
func TestPyramidsShieldsTheNextDestruction(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	pyramids := mustCorpusCard(t, reg, "Pyramids")
	mountain := mustCorpusCard(t, reg, "Mountain")
	rain := mustCorpusCard(t, reg, "Stone Rain")
	e, cfg := tokenReplGame(t, 7, pyramids, mountain, rain, rain)
	first := moveSeededCard(t, e, 0, rain, state.ZHand)
	second := moveSeededCard(t, e, 0, rain, state.ZHand)
	art := moveSeededCard(t, e, 0, pyramids, state.ZBattlefield)
	land := moveSeededCard(t, e, 0, mountain, state.ZBattlefield)
	e.emit(events.Event{Kind: events.Damage, Obj: land, Amount: 1})
	addMana(t, e, 0, "CC")
	idx := -1
	for _, o := range e.Pending().Options {
		if o.Obj == art && o.Kind == "ability" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatal("no activation option for Pyramids")
	}
	submitChoices(t, e, idx)
	if d := e.Pending(); d == nil || d.Kind != decision.KModes {
		t.Fatalf("want the mode ask, got %+v", e.Pending())
	}
	submitChoices(t, e, modeOptionContaining(t, e.Pending(), "next time"))
	submitChoices(t, e, indexOfObjOption(e.Pending(), land))
	passUntilStackEmpty(t, e, 20)

	addMana(t, e, 0, "RRR")
	castDestroyAt(t, e, first, land)
	if o := e.G.Obj(land); o.Zone != state.ZBattlefield || o.Damage != 0 {
		t.Fatalf("land zone %s, damage %d after the shielded destruction", o.Zone, o.Damage)
	}
	addMana(t, e, 0, "RRR")
	castDestroyAt(t, e, second, land)
	if z := e.G.Obj(land).Zone; z == state.ZBattlefield {
		t.Fatal("the shield covers only the next destruction")
	}
	replayCheck(t, e, cfg)
}
