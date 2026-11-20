package oracle

import (
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// The filter words kgen lowers that read no resolution context: a type, a
// colour, a controller, a state flag, a numeric bound, a source-relative flag
// that a probe's source answers. Each is read in every zone gorge's matcher
// and the kernel's card matcher can both answer; a permanent-only word names
// AtomBattlefield.
func init() {
	for _, w := range []string{
		"Card", "Creature", "Land", "Artifact", "Enchantment", "Planeswalker", "Instant", "Sorcery", "Permanent", "Battle",
		"Card.nonCreature", "Card.nonLand", "Card.nonArtifact", "Card.nonEnchantment", "Card.nonPlaneswalker",
		"Card.Legendary", "Card.nonLegendary", "Card.Basic", "Card.nonBasic",
		"Card.White", "Card.Blue", "Card.Black", "Card.Red", "Card.Green", "Card.Colorless", "Card.nonColorless",
		"Card.nonWhite", "Card.nonBlue", "Card.nonBlack", "Card.nonRed", "Card.nonGreen",
		"Card.MultiColor", "Card.MonoColor",
		"Card.YouCtrl", "Card.OppCtrl", "Card.YouOwn", "Card.OppOwn",
		"Card.token", "Card.nonToken",
		"Creature.cmcEQ0", "Card.cmcEQ1", "Card.cmcEQ2", "Card.cmcEQ3", "Card.cmcLE2", "Card.cmcGE3", "Card.cmcGT1", "Card.cmcLT2",
		"Card.Historic", "Card.HasCounters",
		// the kernel's card matcher answers the numeric and Outlaw words off the
		// battlefield too (predCardPT, predCardKvf), as gorge's MatchesSpecInZone does
		"Card.Outlaw",
	} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomAnyZone})
	}
	for _, w := range []string{
		"Creature.powerLE2", "Creature.powerGE3", "Creature.powerGT2", "Creature.powerLT1", "Creature.toughnessLE2", "Creature.toughnessGE3",
	} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomAnyZone, Skip: skipOffZoneCDAPT})
	}
	for _, w := range []string{
		"Card.tapped", "Card.untapped", "Card.attacking", "Card.blocking", "Card.!attacking",
		"Creature.powerEQ0", "Creature.powerEQ1", // kgen refuses power EQ in a card zone
		"Card.modified", "Card.monstrous", "Card.faceDown", "Card.withFlying", "Card.withoutFlying",
		"Card.Other", "Card.Self", "Card.ThisTurnEntered", "Card.enchanted", "Card.equipped",
		"Card.EquippedBy", "Card.EnchantedBy", "Card.hasManaAbility",
	} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomBattlefield})
	}
}

// skipOffZoneCDAPT: off the battlefield, an object whose power or toughness is
// not its printed number (a characteristic-defining ability). The kernel's card
// matcher models the CDA there; gorge's zone matcher reads the printed face
// (rules.specCtx binds no CDA table), so a P/T word is not compared on it
// (ticket gorge-zone-cda-pt).
func skipOffZoneCDAPT(o *state.Object, d rules.Derived) bool {
	f := o.Face()
	return f != nil && o.Zone != state.ZBattlefield && (int(d.Power) != f.Power() || int(d.Toughness) != f.Toughness())
}
