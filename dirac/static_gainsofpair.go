package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StallGainsOf as a pair refusal (kop.PairGuard): a GainsAbilitiesOf$ static
// beside a creature whose activated ability the gained-ability walk does not
// model (gainedModelled: a battlefield one with a once-per-game, once-each-turn
// or loyalty cost, exhaust or a host cost). An off-battlefield one (cycling,
// channel) is skipped by the offer walk (gainedOffer, CR 113.6b: the copy
// lives on a battlefield recipient, so it functions nowhere); an index past
// 63 stays a runtime stall, not a refusal: recorded games hold a Marvin beside
// a cycling creature. A mana ability is neither:
// it is never a priority option, so the offer walk skips it (gainedOffer),
// and the matchup plays. gorge counts a gained once-limited activation per (recipient, foreign
// object, ability index) over the recipient's stint (legal_granted.go
// gainedActivationsUsed); the kernel holds no such record, and the walk stalls
// the moment the recipient has the foreign creature in range
// (gainedOffer). The matchup is refused at its start instead of
// stalling mid-game. Only creatures are read: every GainsOf filter the
// lowering accepts picks creatures (Marvin, Experiment Kraj, Robaran
// Mercenaries, Necrotic Ooze, Thranduil, Dark Impostor, Patchwork Crawler)
// or Foods (Hazel's Brewmaster), whose abilities the walk models.
//
// The second refusal is a GainsOf static of any lender zone beside a creature
// with a tap-another-permanent mana ability (Citanul Stalwart's "{T}, Tap an
// untapped artifact or creature you control: Add one mana of any color"): the
// recipient gains it (CR 613.1f), and gorge's payment census pays a cost with
// the recipient's {T} and a second permanent's tap (CR 605.1a, 602.2), while the
// kernel's grant view prices only a plain {T} (prepGrantCost: a CostTapX grant
// is no tap source), so a Dark Impostor with the Stalwart exiled under it offers
// an activation the kernel counts one mana short of. The tap-another search
// models a printed source, never a gained one; the matchup is refused.

// gainsOfPairOn: t holds both halves of the GainsOf refusal (prep.gainsOfPair).
func gainsOfPairOn(t *kop.Table) bool {
	return gainsOfOnceLimitedPair(t) || gainsOfTapXManaPair(t)
}

// gainsOfOnceLimitedPair: a broad GainsOf lender beside a creature whose
// battlefield activated ability carries a limit the kernel keeps no record of.
func gainsOfOnceLimitedPair(t *kop.Table) bool {
	return cardsWith(t, func(c *kop.CardDef) bool { return hasBroadGainsOf(t, c) }) &&
		cardsWith(t, gainsOfUnmodelledCreature)
}

// gainsOfTapXManaPair: a GainsOf static of any lender zone beside a creature
// with a tap-another mana ability a recipient would gain.
func gainsOfTapXManaPair(t *kop.Table) bool {
	return cardsWith(t, hasGainsOfStatic) && cardsWith(t, gainsOfTapXManaCreature)
}

// hasGainsOfStatic: c has a Continuous static with a GainsOf filter, whatever
// zone it reads.
func hasGainsOfStatic(c *kop.CardDef) bool {
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.GainsOf != 0 {
			return true
		}
	}
	return false
}

// gainsOfTapXManaCreature: c is a creature with a printed mana ability whose
// cost taps another permanent (kop.CostTapX).
func gainsOfTapXManaCreature(c *kop.CardDef) bool {
	if c.Types&kop.TCreature == 0 {
		return false
	}
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && a.Cost.Flags&kop.CostTapX != 0 {
			return true
		}
	}
	return false
}

// hasBroadGainsOf: c has a Continuous static whose GainsOf filter reads the
// battlefield and puts no counter condition on the lender: Marvin, Murderous
// Mimic and Robaran Mercenaries borrow from any creature they control, so a
// lender with an unmodelled ability is in range as soon as it enters. A
// graveyard or exile lender (Necrotic Ooze, Dark Impostor, Hazel's Brewmaster)
// or a counter-gated one (Experiment Kraj) needs more than that, and stays a
// runtime stall: recorded games hold such a card beside a once-limited creature
// that never comes into range.
func hasBroadGainsOf(t *kop.Table, c *kop.CardDef) bool {
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.GainsOf == 0 || a.Static.Zone != kop.ZoneNone {
			continue
		}
		switch t.Filters[a.Static.GainsOf].Pred {
		case kop.PredHasCounter, kop.PredCountersGE, kop.PredCountersLT, kop.PredHasCounters, kop.PredNoCounters:
			continue
		}
		return true
	}
	return false
}

// gainsOfUnmodelledCreature: c is a creature with an active (non-mana)
// battlefield activated ability the gained-ability walk stalls on for its
// activation limit, exhaust or host cost. A mana ability is left out: it is
// not a priority option, so the offer walk skips it (gainedOffer); the
// payment planner's gained-source limit is not a refusal. A hand-zone one
// (cycling) is no stall at all: the offer walk skips it (gainedOffer, CR
// 113.6b -- the gained copy lives on a battlefield recipient, so it
// functions nowhere).
func gainsOfUnmodelledCreature(c *kop.CardDef) bool {
	if c.Types&kop.TCreature == 0 {
		return false
	}
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbActivated || a.AFlags&kop.AfLatent != 0 || a.ActZone != kop.ZoneNone {
			continue
		}
		if !gainedModelled(a, j) {
			return true
		}
	}
	return false
}

// gainsOfStallPair is StallGainsOf's refusal text and halves.
func gainsOfStallPair(t *kop.Table) (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	if !gainsOfOnceLimitedPair(t) {
		return "a GainsAbilitiesOf$ static beside a creature with a tap-another-permanent mana ability: gorge's payment census pays a gained one with the recipient's {T} and a second permanent's tap, the kernel's grant view prices only a plain {T}",
			func(c *kop.CardDef, _ int) bool { return hasGainsOfStatic(c) },
			func(c *kop.CardDef, _ int) bool { return gainsOfTapXManaCreature(c) }
	}
	return "a GainsAbilitiesOf$ static beside a creature with a battlefield activated ability with a once-per-game, once-each-turn, loyalty, exhaust or host cost: gorge counts a gained once-limited activation per recipient and foreign object, the kernel keeps no such record",
		func(c *kop.CardDef, _ int) bool { return hasBroadGainsOf(t, c) },
		func(c *kop.CardDef, _ int) bool { return gainsOfUnmodelledCreature(c) }
}
