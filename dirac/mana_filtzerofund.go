package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A filter whose dynamic amount resolves to zero (Three Tree City's
// "{2}, {T}: Add an amount of mana of any one color equal to the number of
// creatures you control of the chosen type" beside no creature of that type)
// is no census gap: gorge's census drops an ability that yields nothing
// (manaAbilityYieldsNothing), and so does scriptFilt. Its card is still a
// covered funding source of gorge's prefix search, and its "activate" option
// there poses gorge's mana-ability ask whenever the floating pool pays the
// filter's fee: the ask is a node of the potentialScriptNodes budget and its {C} answer is
// one child. The filter's own answer is no branch: it pays the fee, asks the
// colour and adds no unit, and gorge's search prunes exactly that answer from
// the ask's list (scriptDropEmptyManaAnswers; rules scriptSearch.visit and
// activations; CR 605.3a). The kernel's funding pass used to tap the card for
// its {C} with no ask, so it spent fewer nodes than gorge and reached a script
// gorge's budget never did (Minamo, School at Water's Edge's "{U}, {T}: Untap
// target legendary permanent" funded by Starfleet Academy's Federation
// filter); walking the answer with the ask overspent the budget the other way
// (Shiv's Embrace's {R} pump at decision 745).

// zeroFiltAsks reports whether funding source sl poses that ask at t: a
// modelled any-colour filter of one dynamic amount, out of the search's gaps,
// usable, yielding nothing, and with its fee floating.
func (fs *filtSearch) zeroFiltAsks(k *prep, t *State, sl Slot) bool {
	return !fs.gaps.has(sl) && zeroFiltTap(k, t, fs.p, sl)
}

// zeroFiltTap is that shape without a search's gaps: the filter search asks
// it of a non-gap, and grantSrch's funding loop asks it of every non-gap
// candidate, whose plain {T}: {C} tap poses no ask of its own.
func zeroFiltTap(k *prep, t *State, p uint8, sl Slot) bool {
	if !k.gen.anyDynFilt || !k.gen.has(t.cardAt(sl), gfFilt) || t.filtFirstOnly(k, sl) {
		return false
	}
	a := t.filtAb(k, sl)
	if a.AFlags&kop.AfManaDynamic == 0 || a.ManaColors != kop.ColAny || comboFiltAb(a) || a.ManaEach {
		return false
	}
	return t.filtUsable(k, sl) && t.filtUnits(k, sl) == 0 && t.filtPaid(k, p, sl)
}
