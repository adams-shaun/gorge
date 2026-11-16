package pickbot

import (
	"math/bits"

	"github.com/adams-shaun/gorge/draft/limited/build"
)

// cmdrLandsCap is a 60-card Commander-draft deck's land ceiling when the
// identity's width adds lands; cmdrLandsForWidth scales it to the deck's size.
const cmdrLandsCap = 24

// cmdrLandsForWidth is a size-card deck's land count for a commander
// identity mask: cmdrLands plus one land per colour beyond two (a wide
// identity needs more sources of each colour), capped at cmdrLandsCap, all
// scaled to size as cmdrLandsFor does.
func cmdrLandsForWidth(size int, mask uint8) int {
	n := min(cmdrLands+max(0, bits.OnesCount8(mask)-2), cmdrLandsCap)
	return (2*n*size + build.CommanderDeckSize) / (2 * build.CommanderDeckSize)
}

// identitySources counts, per colour of mask, the deck's mana sources: the
// lands and the commanders and spells that are sources of it (cmdrCard.src,
// so an on-identity rock or dork counts for the colours it produces, an
// any-colour one for every colour of the identity). Colours outside mask
// count nothing.
func identitySources(cs []cmdrCard, mask uint8, groups ...[]int) (n [5]int) {
	for _, g := range groups {
		for _, i := range g {
			for c := 0; c < 5; c++ {
				if mask&cs[i].src&(1<<c) != 0 {
					n[c]++
				}
			}
		}
	}
	return n
}
