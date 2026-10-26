package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// twoPilesValidCards is the battlefield shape's ValidCards$ spec, read once
// here: the permanents (of the pool player) the piles are made of.
func twoPilesValidCards(sa *cards.SA) string {
	return strings.TrimSpace(sa.ParamStr(cards.PKValidCards))
}
