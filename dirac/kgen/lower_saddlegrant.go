package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Saddle N granted by a layer-6 AddKeyword$ (Jandor, Fortuned Traveler's
// "each Beast, Camel, Horse, Elephant, and Wolf creature you control ... has
// saddle 2"; CR 702.171a). gorge mints the granted keyword's activated ability
// with cards.GrantedKeywordAbility, the printed kwSaddle expander's body, so
// the grant lowers like the printed line's AB$ AlterAttribute: a latent
// ability the static lists among its Grant entries, one per granting static
// (each Saddle instance is its own ability, CR 702.171a, 113.2c).

// splitSaddleGrant takes every "Saddle:<N>" term out of a `&` keyword list:
// the rest of the list and the terms taken.
func splitSaddleGrant(kw string) (string, []string) {
	var rest, saddles []string
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if cards.KeywordHead(k) == "Saddle" {
			saddles = append(saddles, k)
			continue
		}
		rest = append(rest, k)
	}
	return strings.Join(rest, " & "), saddles
}

// saddleGrantAbility lowers one granted "Saddle:<N>" term as a latent ability
// and returns its index.
func (lc *lowerCtx) saddleGrantAbility(line string) (int, error) {
	sa := cards.GrantedKeywordAbility(line)
	if sa == nil || sa.Kind != "AB" {
		return 0, refuse("keyword %q", line)
	}
	return lc.grantedKeywordLatent(sa)
}
