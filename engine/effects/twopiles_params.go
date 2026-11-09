package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// twoPilesDefinedPiles is DefinedPiles$ split on the FIRST comma (Forge's
// split(",", 2)): the two pile selectors, trimmed. ok is false when the value
// is absent or does not name two non-empty selectors. The split itself lives
// in cards.TwoPilesDefinedPiles, shared with the coverage gate.
func twoPilesDefinedPiles(sa *cards.SA) (a, b string, ok bool) {
	return cards.TwoPilesDefinedPiles(sa)
}

// twoPilesValidCards is the battlefield shape's ValidCards$ spec, read once
// here: the permanents (of the pool player) the piles are made of.
func twoPilesValidCards(sa *cards.SA) string {
	return strings.TrimSpace(sa.ParamStr(cards.PKValidCards))
}

// twoPilesDefinedPilesValue is the raw DefinedPiles$ value (empty: the
// ordinary split shapes).
func twoPilesDefinedPilesValue(sa *cards.SA) string {
	return strings.TrimSpace(sa.ParamStr(cards.PKDefinedPiles))
}

// twoPilesChooserSpec is the DefinedPiles arm's chooser selector: Chooser$,
// else Defined$, else empty (the controller, Forge's getTargetPlayers default).
func twoPilesChooserSpec(sa *cards.SA) string {
	if v := strings.TrimSpace(sa.ParamStr(cards.PKChooser)); v != "" {
		return v
	}
	return DefinedRefOf(sa).Text
}
