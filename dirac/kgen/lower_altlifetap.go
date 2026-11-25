package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// altLifeOrTapShape: the built alternative cost is paying life alone (Snuff
// Out's PayLife<4>) or tapping untapped permanents alone (Ramosian Rally's
// tapXType<1/Creature>), CR 118.9: kernel dirac.altLifeCost / altTapCost. The
// card is gated by cardBuilder.altSacOK (an instant, sorcery or spell-less
// creature), as the sacrifice and return alternatives are.
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

// altPickShape is the built alternative cost shapes the kernel's altPickCost
// offers: a sacrifice (CostSac, alone or with life), a return, a hand exile,
// a discard, life alone, or taps. The mana part may be nonzero (a
// Borderpost's {1} plus a Return, Sephara's {W} plus taps).
func altPickShape(c *kop.Cost) bool {
	sac := c.Flags&^(kop.CostSac|kop.CostPayLife) == 0 && c.Flags&kop.CostSac != 0 && c.N > 0
	ret := c.Flags == kop.CostReturn && c.N > 0
	return sac || ret || altHandExileShape(c) || altSacGatedShape(c)
}
