package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// altLifeOrTapShape: the built alternative cost is paying life alone (Snuff
// Out's PayLife<4>) or tapping untapped permanents alone (Ramosian Rally's
// tapXType<1/Creature>), CR 118.9: kernel dirac.altLifeCost / altTapCost. The
// card is gated by cardBuilder.altSacOK (an instant, sorcery or spell-less
// creature), as the sacrifice and return alternatives are.
func altLifeOrTapShape(c *kop.Cost) bool {
	if c.Mana != (kop.ManaCost{}) || c.N == 0 {
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
