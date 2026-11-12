package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 509.1c / 613.1f: a Continuous static's AddHiddenKeyword$ "All creatures
// able to block CARDNAME do so." (Lure, Gift of the Deity, Seton's Desire,
// Stone-Tongue Basilisk) is a layer-6 keyword grant, read per attacker like
// the printed keyword and the Pump KW$ grant. It was dropped: the static
// registered no continuous effect, so the enchanted creature was no lure.
func TestLureStaticGrantReachesTheEnchantedCreature(t *testing.T) {
	t.Parallel()
	lure := "Name:Lure Fixture\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Creature | AddHiddenKeyword$ All creatures able to block CARDNAME do so. | Description$ x\nOracle:x\n"
	e, _, ra := newFixtureDeck(t, 64, lure)
	bear := putToken(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", state.ZBattlefield)
	other := putToken(t, e, 1, "Name:Other Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", state.ZBattlefield)
	if combat.DerivedHiddenFlags(asBoard(e), bear).AllMustBlock {
		t.Fatal("precondition: the bear is a lure before the Aura")
	}
	addMana(t, e, 0, "G")
	castFirst(t, e, "cast")
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(ra).AttachedTo != bear {
		t.Fatalf("precondition: the Aura is on %d, not the bear %d", e.G.Obj(ra).AttachedTo, bear)
	}
	if !combat.DerivedHiddenFlags(asBoard(e), bear).AllMustBlock {
		t.Fatal("the enchanted creature is not a lure")
	}
	if combat.DerivedHiddenFlags(asBoard(e), other).AllMustBlock {
		t.Fatal("an unenchanted creature is a lure")
	}
}
