package effects

import "github.com/adams-shaun/gorge/cards"

func init() { Register("PermanentCreature", effPermanentCreature) }

// effPermanentCreature registers SP$ PermanentCreature (Forge's API for "this
// line IS the cast of a creature card" -- 100+ corpus files, Dargo, the
// Shipwrecker and the other pitch/alternate-cost creatures among them). The
// cast itself is the rules engine's ordinary card-cast flow: the stack object
// is the CARD, beginCast folds the SP line's Cost$ additional parts
// (withSpellAbilityExtras), resolution moves the creature onto the
// battlefield, and an ETB trigger -- not a resolution body -- is what a
// creature's own text runs on. So the resolution body is genuinely empty; the
// registration is what retires the primitive from the coverage audit (before
// it, resolving such a cast emitted an "unimplemented API" Note on the way to
// the same battlefield move). A face whose SP line is this API but that is NOT
// a permanent face would be a script error, and the empty body degrades to a
// harmless no-op for it.
//
// _ is the effect signature: the SA's parameters (Cost$, AILogic$,
// SpellDescription$) are all cast-time concerns the rules package already
// reads.
func effPermanentCreature(h Host, c *Ctx, sa *cards.SA) {
	// The cast itself is the whole effect; see the doc above.
}
