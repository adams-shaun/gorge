package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
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
	return strings.TrimSpace(cards.TwoPilesBattlefieldValid(sa))
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

// twoPilesRemembersChosen is RememberChosen$ True: the chosen pile joins the
// resolution's Remembered set (cards.TwoPilesRemembersChosen).
func twoPilesRemembersChosen(sa *cards.SA) bool {
	return cards.TwoPilesRemembersChosen(sa)
}

// twoPilesKeepsRemembered is KeepRemembered$ present (Forge's hasParam: any
// value counts): the resolution leaves the source's remembered list alone
// instead of clearing it once both pile bodies ran (Hostile Negotiations'
// YouChoose keeps it for TheyChoose).
func twoPilesKeepsRemembered(sa *cards.SA) bool {
	if sa == nil {
		return false
	}
	return sa.HasParam(cards.PKKeepRemembered)
}

// twoPilesFixedSet is the non-battlefield card set a DefinedCards$ spec names,
// in the deterministic order: Remembered (minus the captured event object),
// Targeted, Imprinted, or the top N cards of the controller's library.
func twoPilesFixedSet(h Host, c *Ctx, spec string) []state.ObjID {
	g := h.Game()
	kind, n := cards.TwoPilesDefinedCards(spec)
	var set []state.Target
	switch kind {
	case cards.TwoPilesSetImprinted:
		ids, _ := twoPilesPileObjs(h, c, spec)
		return ids
	case cards.TwoPilesSetTop:
		lib := zoneOf(g, state.ZLibrary, c.Controller)
		if len(lib) > n {
			lib = lib[:n]
		}
		return append([]state.ObjID(nil), lib...)
	case cards.TwoPilesSetTargeted:
		set = objectsOf(c.Targets)
	default:
		set = objectsOf(rememberedExcludingCapture(h, c))
	}
	ids := make([]state.ObjID, 0, len(set))
	for _, t := range set {
		if o := g.Obj(t.Obj); o != nil {
			ids = append(ids, o.ID)
		}
	}
	return ids
}
