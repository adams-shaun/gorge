package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPotentialPrefixScriptAnswersAnOpponentsTriggerOrder: a {G} dork
// enchanted by two of the opponent's "when enchanted creature becomes
// tapped, destroy it" Auras. Tapping it for mana sets off both triggers,
// which wait until its controller would next get priority (CR 603.3b); the
// opponent orders them first (CR 603.3b), and the {G} is already in the pool
// (CR 605.3a). The prefix search answers that order as offered, so the
// {1}{G} creature stays castable rather than census_incomplete.
func TestPotentialPrefixScriptAnswersAnOpponentsTriggerOrder(t *testing.T) {
	t.Parallel()
	const (
		spellSrc = "Name:Order Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
		forest   = "Name:Order Grove\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ x\nOracle:x\n"
		dork     = "Name:Order Dork\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ x\nOracle:x\n"
		aura     = "Name:Order Frost\nManaCost:3 U\nTypes:Enchantment Aura\nK:Enchant:Creature\nA:SP$ Attach | Cost$ 3 U | ValidTgts$ Creature | AILogic$ Curse\n" +
			"T:Mode$ Taps | ValidCard$ Card.AttachedBy | TriggerZones$ Battlefield | Execute$ TrigFrostDestroy | TriggerDescription$ x\n" +
			"SVar:TrigFrostDestroy:DB$ Destroy | Defined$ TriggeredCardLKICopy\nOracle:x\n"
	)
	build := func(auras int) PotentialPlan {
		e, _, spell := newFixtureDeck(t, 9995, spellSrc)
		onBoard(t, e, 0, forest)
		elf := onBoardReady(t, e, 0, dork)
		for i := 0; i < auras; i++ {
			a := onBoard(t, e, 1, aura)
			e.emit(events.Event{Kind: events.Attach, Obj: a, IDs: []state.ObjID{elf}})
		}
		toMain1(t, e)
		moveToHand(t, e, spell)
		ppPriority(t, e)
		n := 0
		for _, id := range e.G.Zone(state.ZBattlefield, 1) {
			if o := e.G.Obj(id); o.Face().Name == "Order Frost" && o.AttachedTo == elf {
				n++
			}
		}
		if o := e.G.Obj(elf); n != auras || o.Zone != state.ZBattlefield || o.Tapped || o.SummonSick {
			t.Fatalf("fixture: %d auras on the dork (want %d), zone %s tapped %v sick %v", n, auras, o.Zone, o.Tapped, o.SummonSick)
		}
		return faceCastPlan(t, e.PotentialPaymentPlans(0), spell, "")
	}
	// Precondition: one aura already makes the dork a census gap the prefix
	// script pays through (no trigger order to answer).
	if one := build(1); one.Reason != "" || len(one.Script) == 0 {
		t.Fatalf("one aura: reason %q detail %q script %d steps, want a prefix script", one.Reason, one.Detail, len(one.Script))
	}
	two := build(2)
	if two.Reason != "" || len(two.Script) == 0 {
		t.Fatalf("two auras: reason %q detail %q script %d steps, want a prefix script past the opponent's trigger order", two.Reason, two.Detail, len(two.Script))
	}
}
