package effects

import "github.com/adams-shaun/gorge/engine/cards"

func init() { Register("PermanentNoncreature", effPermanentNoncreature) }

// effPermanentNoncreature registers SP$ PermanentNoncreature (Forge's API for
// "this line IS the cast of a noncreature permanent card" -- Bridge from
// Below among the corpus files). The cast itself is the rules engine's
// ordinary card-cast flow: the stack object is the CARD, beginCast folds the
// SP line's Cost$ additional parts (withSpellAbilityExtras), and resolution
// moves the permanent onto the battlefield; the card's own text (Bridge from
// Below's two graveyard triggers) runs as ordinary triggers, not as a
// resolution body. So the resolution body is genuinely empty; the
// registration is what retires the primitive from the coverage audit (before
// it, resolving such a cast emitted an "unimplemented API" Note on the way to
// the same battlefield move). A face whose SP line is this API but that IS a
// creature or non-permanent face would be a script error, and the empty body
// degrades to a harmless no-op for it.
//
// _ is the effect signature: the SA's parameters (Cost$, AILogic$,
// SpellDescription$) are all cast-time concerns the rules package already
// reads.
func effPermanentNoncreature(h Host, c *Ctx, sa *cards.SA) {
	// The cast itself is the whole effect; see the doc above.
}
