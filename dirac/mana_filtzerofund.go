package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A filter whose dynamic amount resolves to zero (Three Tree City's
// "{2}, {T}: Add an amount of mana of any one color equal to the number of
// creatures you control of the chosen type" beside no creature of that type)
// is no census gap: gorge's census drops an ability that yields nothing
// (manaAbilityYieldsNothing), and so does scriptFilt. Its card is still a
// covered funding source of gorge's prefix search, and its "activate" option
// there poses gorge's mana-ability ask whenever the floating pool pays the
// filter's fee: the ask is a node of the 160-node budget, its {C} answer is
// one child, and the filter's answer is another, which pays the fee, asks the
// colour (a node) and visits each of its colour answers though none adds a
// unit (rules scriptSearch.visit and activations; CR 605.3a). The kernel's
// funding pass used to tap the card for its {C} with no ask, so it spent
// fewer nodes than gorge and reached a script gorge's budget never did
// (Minamo, School at Water's Edge's "{U}, {T}: Untap target legendary
// permanent" funded by Starfleet Academy's Federation filter).

// zeroFiltAsks reports whether funding source sl poses that ask at t: a
// modelled any-colour filter of one dynamic amount, out of the search's gaps,
// usable, yielding nothing, and with its fee floating.
func (fs *filtSearch) zeroFiltAsks(k *prep, t *State, sl Slot) bool {
	if !k.gen.anyDynFilt || !k.gen.has(t.cardAt(sl), gfFilt) || fs.gaps.has(sl) || t.filtFirstOnly(k, sl) {
		return false
	}
	a := t.filtAb(k, sl)
	if a.AFlags&kop.AfManaDynamic == 0 || a.ManaColors != kop.ColAny || comboFiltAb(a) || a.ManaEach {
		return false
	}
	return t.filtUsable(k, sl) && t.filtUnits(k, sl) == 0 && t.filtPaid(k, fs.p, sl)
}

// zeroFiltColours is the zero filter's answer at sl: its colour ask, then
// each colour in gorge's order (needFirst), every one a funded step that pays
// the fee and adds nothing.
func (fs *filtSearch) zeroFiltColours(k *prep, t *State, sl Slot, funded int, gapUsed bool) bool {
	if !fs.tick() { // the colour ask
		return false
	}
	var order [5]uint8
	n := manaColourOrder(t.filtAb(k, sl), &order)
	needFirst(order[:n], fs.need)
	for _, c := range order[:n] {
		if fs.try(k, t, filtStep{sl: sl, ans: 1 + c, gap: true}, funded, gapUsed) {
			return true
		}
	}
	return false
}
