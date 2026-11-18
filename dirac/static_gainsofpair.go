package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StallGainsOf as a pair refusal (kop.PairGuard): a GainsAbilitiesOf$ static
// beside a creature whose activated ability the gained-ability walk does not
// model (gainedModelled: a mana ability, a once-per-game, once-each-turn or
// loyalty one, exhaust, a host cost, an off-battlefield one, or an index past
// 63). gorge counts a gained once-limited activation per (recipient, foreign
// object, ability index) over the recipient's stint (legal_granted.go
// gainedActivationsUsed); the kernel holds no such record, and the walk stalls
// the moment the recipient has the foreign creature in range
// (gainedOffer). The matchup is refused at its start instead of
// stalling mid-game. Only creatures are read: every GainsOf filter the
// lowering accepts picks creatures (Marvin, Experiment Kraj, Robaran
// Mercenaries, Necrotic Ooze, Thranduil, Dark Impostor, Patchwork Crawler)
// or Foods (Hazel's Brewmaster), whose abilities the walk models.

// prepGainsOfPair marks a table with both halves of the GainsOf refusal.
func prepGainsOfPair(t *kop.Table, g *genPrep) {
	g.gainsOfPair = cardsWith(t, hasGainsOfStatic) && cardsWith(t, gainsOfUnmodelledCreature)
}

// hasGainsOfStatic: c has a Continuous static with a GainsOf filter.
func hasGainsOfStatic(c *kop.CardDef) bool {
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.GainsOf != 0 {
			return true
		}
	}
	return false
}

// gainsOfUnmodelledCreature: c is a creature with an active activated or mana
// ability the gained-ability walk stalls on.
func gainsOfUnmodelledCreature(c *kop.CardDef) bool {
	if c.Types&kop.TCreature == 0 {
		return false
	}
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbActivated && a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if !gainedModelled(a, j) {
			return true
		}
	}
	return false
}

// gainsOfStallPair is StallGainsOf's refusal text and halves.
func gainsOfStallPair() (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	return "a GainsAbilitiesOf$ static beside a creature with a mana, once-per-game, once-each-turn, loyalty, exhaust, host-cost or off-battlefield activated ability: gorge counts a gained once-limited activation per recipient and foreign object, the kernel keeps no such record",
		func(c *kop.CardDef, _ int) bool { return hasGainsOfStatic(c) },
		func(c *kop.CardDef, _ int) bool { return gainsOfUnmodelledCreature(c) }
}
