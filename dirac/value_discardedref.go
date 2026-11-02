package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// RefDiscarded (gorge cr-discarded-cost-value-ref,
// refTargetsDiscarded over Ctx.Paid.Discarded): the card a spell's own
// Discard<1/Card> cost discarded, which Grab the Prize's
// Discarded$Valid Card.nonLand reads where the card now is. kgen lowers it
// only for a spell whose cost discards exactly one chosen card; payPicks
// records that card in the spell entry's trigObj (a spell's trigObj is
// otherwise unused but by a sneak cast's attack, which never discards), and
// a copy keeps it (CR 707.10, gorge inheritCopyRecord). 0 B.

// prepDiscarded sets anyDiscRef: some expression reads RefDiscarded.
func prepDiscarded(t *kop.Table, g *genPrep) {
	for i := range t.Exprs {
		if t.Exprs[i].Ref == kop.RefDiscarded {
			g.anyDiscRef = true
			return
		}
	}
}

// discardedObj is entry e's cost-discarded card, NoObj without one.
func discardedObj(e *stackEntry) Obj {
	if e.kind != seSpell || e.trigPl == trigPlSneak {
		return NoObj
	}
	return e.trigObj
}
