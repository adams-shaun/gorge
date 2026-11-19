// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwTransfigure(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.71: transfigure is a sorcery-speed activation of the creature
	// itself: "{cost}, Sacrifice this creature: Search your library for a
	// creature card with the same mana value as this creature, put that card
	// onto the battlefield, then shuffle." The searched mana value is the
	// source card's printed one (the ChangeZone shuffle is intrinsic), so
	// the ability mirrors kw_transmute.go's shape with a battlefield
	// activation zone, a sacrifice half-cost and an enters-the-battlefield
	// destination. Cost arrives already trimmed (keywords.go trims the param).
	cost := param
	sa, _ := parseSA("", "AB$ ChangeZone | Cost$ "+cost+" Sac<1/CARDNAME> | ActivationZone$ Battlefield | SorcerySpeed$ True | Origin$ Library | Destination$ Battlefield | ChangeType$ Creature.cmcEQ"+cmcSuffix(f)+" | ChangeNum$ 1 | Keyword$ Transfigure | SpellDescription$ Transfigure "+cost)
	if sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwTransfigure, "Transfigure") }
