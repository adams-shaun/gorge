package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// RefChosenEnc (gorge's Ctx.Paid.Revealed over the cast-cost PAID list,
// refTargetsCodeOf's RevealedCards spelling): the object a spell's own
// CloseEncounter ChooseCard additional cost chose, which the spell body's X
// (RevealedCards\$CardPower) reads — its power where the object now is,
// live on the battlefield, the printed face in exile (gorge
// count_ref_property.go's CardPower; CR 601.2b). kgen lowers the read only
// for a spell whose own additional cost is the choose part; the payment
// records the chosen object in the spell entry's trigObj, as RefDiscarded
// does (a spell's trigObj is otherwise unused but by a sneak cast's attack,
// which never pays a ChooseCard cost, and a copy keeps it, CR 707.10, gorge
// inheritCopyRecord). 0 B.

// prepChosenEnc sets anyEncRef: some expression reads RefChosenEnc.
func prepChosenEnc(t *kop.Table, g *genPrep) {
	for i := range t.Exprs {
		if t.Exprs[i].Ref == kop.RefChosenEnc {
			g.anyEncRef = true
			return
		}
	}
}

// chosenEncObj is entry e's cost-chosen object, NoObj without one.
func chosenEncObj(e *stackEntry) Obj {
	if e.kind != seSpell || e.trigPl == trigPlSneak {
		return NoObj
	}
	return e.trigObj
}
