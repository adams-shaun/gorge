package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func ofsPrice(e *Engine, id state.ObjID, scope costScope, targets []state.Target, mana string) int32 {
	return e.costModifiersForTargets(0, id, scope, targets).Apply(e.parseCost(mana)).CMC()
}

func ofsPush(e *Engine, id state.ObjID) {
	e.emit(events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
}

func ofsWant(t *testing.T, label string, got, want int32) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: CMC=%d want %d", label, got, want)
	}
}

// TestOnlyFirstSpellValidSpellKind pins Melek's ValidSpell$ Spell.Instant,
// Spell.Sorcery gate on the OnlyFirstSpell$ tracking: an earlier creature spell
// does not spend the discount, an earlier instant does.
func TestOnlyFirstSpellValidSpellKind(t *testing.T) {
	t.Parallel()
	instantSrc := "Name:Quick\nManaCost:3 U\nTypes:Instant\nOracle:x\n"
	creatureSrc := "Name:Slow\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e := handEngine(t, card(t, instantSrc), card(t, instantSrc), card(t, creatureSrc))
	melek := e.G.AddObject(corpusAlternativeCard(t, "Melek, Reforged Researcher"), 0)
	melek.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{melek.ID})
	hand := e.G.Zone(state.ZHand, 0)
	priced, second, creature := hand[0], hand[1], hand[2]
	if e.G.Obj(melek.ID).Zone != state.ZBattlefield || !e.G.Obj(priced).Face().IsInstant() ||
		!e.G.Obj(creature).Face().IsCreature() || e.G.Active != 0 {
		t.Fatal("precondition: Melek on battlefield, priced is an instant, earlier is a creature, seat 0 active")
	}
	ofsWant(t, "baseline", ofsPrice(e, priced, spellScope(""), nil, "3 U"), 1)
	ofsPush(e, creature)
	ofsWant(t, "after creature", ofsPrice(e, priced, spellScope(""), nil, "3 U"), 1)
	ofsPush(e, second)
	ofsWant(t, "after instant", ofsPrice(e, priced, spellScope(""), nil, "3 U"), 4)
}

// TestOnlyFirstSpellValidSpellKicked pins the cast-mode branch: only an
// earlier KICKED cast spends a ValidSpell$ Spell.Kicked discount.
func TestOnlyFirstSpellValidSpellKicked(t *testing.T) {
	t.Parallel()
	pricedSrc := "Name:Priced\nManaCost:2 U\nTypes:Sorcery\nOracle:x\n"
	creatureSrc := "Name:Slow\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	grantorSrc := "Name:Grantor\nManaCost:1\nTypes:Creature Bear\nPT:1/1\n" +
		"S:Mode$ ReduceCost | EffectZone$ Battlefield | Type$ Spell | OnlyFirstSpell$ True | ValidSpell$ Spell.Kicked | Amount$ 2 | Description$ x\nOracle:x\n"
	e := handEngine(t, card(t, pricedSrc), card(t, creatureSrc))
	g := e.G.AddObject(card(t, grantorSrc), 0)
	g.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{g.ID})
	hand := e.G.Zone(state.ZHand, 0)
	priced, prior := hand[0], hand[1]
	if e.G.Obj(g.ID).Zone != state.ZBattlefield || priced == prior {
		t.Fatal("precondition: grantor on battlefield, distinct priced and prior spells")
	}
	ofsWant(t, "baseline", ofsPrice(e, priced, spellScope("kicked"), nil, "2 U"), 1)
	ofsPush(e, prior)
	ofsWant(t, "after unkicked", ofsPrice(e, priced, spellScope("kicked"), nil, "2 U"), 1)
	e.emit(events.Event{Kind: events.CastInfo, Obj: prior, Counter: events.FlagsString(state.FlagKicked)})
	ofsWant(t, "after kicked", ofsPrice(e, priced, spellScope("kicked"), nil, "2 U"), 3)
}

// TestOnlyFirstSpellValidSpellIsTargeting pins the target branch with Head of
// the Class: only an earlier spell that targeted a creature spends it.
func TestOnlyFirstSpellValidSpellIsTargeting(t *testing.T) {
	t.Parallel()
	spellSrc := "Name:Wounded Spell\nManaCost:W B\nTypes:Sorcery\n" +
		"A:SP$ Pump | ValidTgts$ Creature | NumAtt$ +1 | NumDef$ +1\nOracle:x\n"
	e := handEngine(t, card(t, spellSrc), card(t, spellSrc))
	head := e.G.AddObject(corpusAlternativeCard(t, "Head of the Class"), 0)
	head.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{head.ID})
	bear := battlePerm(t, e, 0, "Name:A Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	hand := e.G.Zone(state.ZHand, 0)
	priced, prior := hand[0], hand[1]
	if e.G.Obj(head.ID).Zone != state.ZBattlefield || e.G.Active != 0 ||
		!e.G.Obj(bear).Face().IsCreature() || priced == prior {
		t.Fatal("precondition: head on battlefield, seat 0 active, bear is a creature, distinct spells")
	}
	tg := []state.Target{{Obj: bear}}
	ofsWant(t, "baseline", ofsPrice(e, priced, spellScope(""), tg, "W B"), 0)
	ofsPush(e, prior)
	ofsWant(t, "after non-targeting cast", ofsPrice(e, priced, spellScope(""), tg, "W B"), 0)
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: prior, IDs: []state.ObjID{bear}})
	ofsWant(t, "after targeting-creature cast", ofsPrice(e, priced, spellScope(""), tg, "W B"), 2)
}
