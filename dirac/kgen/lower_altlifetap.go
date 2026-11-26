package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// altLifeOrTapShape: the built alternative cost is paying life alone (Snuff
// Out's PayLife<4>) or tapping untapped permanents alone (Ramosian Rally's
// tapXType<1/Creature>), CR 118.9: kernel dirac.altLifeCost / altTapCost. The
// card is gated by cardBuilder.altSacOK (an instant, sorcery, spell-less
// permanent, or an Aura whose tap-alone alternative sits beside its one pure
// mana spell ability, Lashknife), as the sacrifice and return alternatives are.
func altLifeOrTapShape(c *kop.Cost) bool {
	if c.N == 0 {
		return false
	}
	return c.Flags == kop.CostPayLife || c.Flags == kop.CostTapX
}

// altSacGatedShape: the alternative cost shapes offered under the same card
// gate as the sacrifice alternatives (cardBuilder.altSacOK): a discard, or
// life or tapping alone.
func altSacGatedShape(c *kop.Cost) bool {
	return altDiscardShape(c) || altLifeOrTapShape(c)
}

// altGraveExileShape: the built alternative cost is exiling N graveyard cards
// matching its filter beside its mana (Squee, Dubious Monarch's
// ExileFromGrave<4/Card.Other>; CR 118.9): the count rides kop.Cost.N, the
// mana beside it, the ExileHand bit clear.
func altGraveExileShape(c *kop.Cost) bool {
	return c.Flags == kop.CostExileGrave && !c.ExileHand && c.N > 0
}

// altPickShape is the built alternative cost shapes the kernel's altPickCost
// offers: a sacrifice (CostSac, alone or with life), a return, a hand exile,
// a graveyard exile (Squee), a discard, life alone, or taps. The mana part
// may be nonzero (a Borderpost's {1} plus a Return, Sephara's {W} plus taps,
// Squee's {3}{R}).
func altPickShape(c *kop.Cost) bool {
	sac := c.Flags&^(kop.CostSac|kop.CostPayLife) == 0 && c.Flags&kop.CostSac != 0 && c.N > 0
	ret := c.Flags == kop.CostReturn && c.N > 0
	return sac || ret || altHandExileShape(c) || altGraveExileShape(c) || altSacGatedShape(c)
}
