package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.103b: a card cast for its bestow cost is an Aura spell, not a
// creature spell, so mana spendable only on creature spells cannot pay the
// bestow cost. The offer priced the card in hand as the creature it is, so a
// creature-only {B} source made the bestowed cast "payable" (Gwenna, Eyes of
// Gaea beside Nyxborn Eidolon). Mana restricted to a quality the Aura spell
// keeps (an Enchantment spell) still pays it.
func TestBestowOfferReadsTheSpellAsAnAura(t *testing.T) {
	const bestowSrc = "Name:Bestow Fixture\nManaCost:1 B\nTypes:Enchantment Creature Spirit\nPT:2/1\nK:Bestow:4 B\n" +
		"S:Mode$ Continuous | AffectedDefined$ Enchanted | AddPower$ 2 | AddToughness$ 1 | Description$ Enchanted creature gets +2/+1.\nOracle:x\n"
	offered := func(t *testing.T, restrict string) bool {
		t.Helper()
		sourceSrc := "Name:Restricted Source\nManaCost:2\nTypes:Creature Elf\nPT:1/1\n" +
			"A:AB$ Mana | Cost$ T | Produced$ B | Amount$ 2 | RestrictValid$ " + restrict + " | SpellDescription$ Add {B}{B}.\nOracle:x\n"
		e, _, card := newFixtureDeck(t, 9822, bestowSrc)
		addMana(t, e, 0, "GGGG") // {4} of the {4}{B}; the source alone can make the {B}
		src := onBoardReady(t, e, 0, sourceSrc)
		onBoardReady(t, e, 0, bestowBearerSrc)
		e.pending = nil
		e.askPriority(0)
		act := -1
		for _, o := range e.Pending().Options {
			if o.Kind == "activate" && o.Obj == src {
				act = o.Index
			}
		}
		if act < 0 {
			t.Fatalf("precondition: source mana ability not offered: %+v", e.Pending().Options)
		}
		submitChoices(t, e, act) // floats the restricted {B}{B}
		e.pending = nil
		e.askPriority(0)
		if z := e.G.Obj(card).Zone; z != state.ZHand {
			t.Fatalf("precondition: fixture in %s, want hand", z)
		}
		for _, o := range castOptions(t, e) {
			if o.Obj == card && o.Mode == "bestowed" {
				return true
			}
		}
		return false
	}
	if !offered(t, "Spell.Enchantment") {
		t.Fatal("precondition: Enchantment-only black mana must pay the bestowed cast")
	}
	if offered(t, "Spell.Creature") {
		t.Fatal("bestowed cast offered on mana only creature spells may spend")
	}
}
