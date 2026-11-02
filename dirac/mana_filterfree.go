package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A filter ({N}, {T}: Add one mana of any color beside the card's free {T}:
// Add {C}) whose fee an activated-ability reducer takes to nothing (Blossoming
// Tortoise on Crystal Grotto, CR 601.2f, 602.2b; gorge manaAbilityCC) is a
// free second tap ability: gorge's census lists one unit for the source, the
// {C} alternative then one per colour, and no paid gap. The kernel's view of
// such a source is the gated-choice shape (mana_gatedchoice.go): the gate-off
// view is the {C} alone, the gate-on view the union C|colours (manaMulti), and
// the gate is "the fee is nothing after the reducers" (manaGateView). A filter
// at that moment is no census gap, so scriptFilt leaves it out.
//
// Only tables holding a reducer pay for it: prepFiltFree marks nothing
// otherwise, and the filter paths read the fee through filtFee as before.

// prepFiltFree builds the fee-paid and fee-free views of every plain filter
// of a table with a StatReduceAbility static.
//
//go:noinline
func prepFiltFree(t *kop.Table, g *genPrep, p *prep) {
	if !g.anyAbReduce {
		return
	}
	for i := range t.Cards {
		id := CardID(i)
		if !g.has(id, gfFilt) || g.has(id, gfManaVar) {
			continue
		}
		c := &t.Cards[i]
		j := int(g.filt(id))
		if j < 0 || firstManaAb(c) == j {
			continue // a filter first and only keeps its own paths (mana_filterfirst.go)
		}
		a := &c.Abilities[j]
		off := p.cards[i]
		if a.AFlags != 0 || a.ManaN > 1 {
			continue // a dynamic amount stays a filter
		}
		u, ok := gatedChoiceUnion(c, j, off)
		if !ok {
			continue
		}
		on := off
		on.manaCol, on.manaMulti = u, true
		g.setFlag(id, gfManaVar|gfManaGate|gfFiltFee)
		g.smallFor(id).manaVar = int16(len(g.varPC))
		g.varPC = append(g.varPC, off, on)
	}
}
