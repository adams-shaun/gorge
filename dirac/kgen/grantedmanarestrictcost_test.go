package kgen

import "testing"

// grantedManaRestrictCostForms pins the granted mana abilities with a spend
// restriction or a cost beyond {T}. A spend restriction on spells (Clement's
// creature spells, Battery Bearer's artifact spells) or on spells and
// abilities (Ambulatory Reactor) lowers: the granted source's tap floats its
// mana in a restricted batch (dirac mana_grantedoffers.go grantApply), as the
// printed restricted sources do. Forgotten Monument's "{T}, Pay 1 life"
// lowers as well (the grant search's step pays the life: dirac
// mana_grantcost.go). Lotus Ring's sacrifice-only ability stays refused, by
// its own reason: the manual mana window cannot activate it.
var grantedManaRestrictCostForms = []formCase{
	{card: "Clement, the Worrywort", form: "a granted Combo restricted to creature spells", lir: "mana GU x1 restrict={types=0x2}"},
	{card: "Ambulatory Reactor", form: "a granted {C} restricted to artifact spells and abilities of artifact sources", lir: "mana C x1 restrict={types=0x4}"},
	{card: "Battery Bearer", form: "a granted ability restricted to artifact spells", lir: "Mana Granted cost=T"},
	{card: "Lotus Ring", form: "a granted sacrifice-only three-mana ability", refused: "granted sacrifice-only mana ability: the kernel's manual mana window cannot activate it"},
	{card: "Forgotten Monument", form: "a granted {T}, Pay 1 life ability", lir: "Mana Granted cost=T Life1"},
}

func TestGrantedManaRestrictCost(t *testing.T) { checkForms(t, grantedManaRestrictCostForms) }
