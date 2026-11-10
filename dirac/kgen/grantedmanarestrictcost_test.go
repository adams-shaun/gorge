package kgen

import "testing"

// grantedManaRestrictCostForms pins the granted mana abilities with a spend
// restriction or a cost beyond {T}. A spend restriction on spells (Clement's
// creature spells, Battery Bearer's artifact spells) or on spells and
// abilities (Ambulatory Reactor) lowers: the granted source's tap floats its
// mana in a restricted batch (dirac mana_grantedoffers.go grantApply), as the
// printed restricted sources do. A cost beyond {T} stays refused: the grant
// search's steps tap the granted source and nothing else, so a sacrifice or a
// life payment would be counted as free.
var grantedManaRestrictCostForms = []formCase{
	{card: "Clement, the Worrywort", form: "a granted Combo restricted to creature spells", lir: "mana GU x1 restrict={types=0x2}"},
	{card: "Ambulatory Reactor", form: "a granted {C} restricted to artifact spells and abilities of artifact sources", lir: "mana C x1 restrict={types=0x4}"},
	{card: "Battery Bearer", form: "a granted ability restricted to artifact spells", lir: "Mana Granted cost=T"},
	{card: "Lotus Ring", form: "a granted sacrifice-only three-mana ability", refused: "granted mana ability whose cost is not {T}"},
	{card: "Forgotten Monument", form: "a granted {T}, Pay 1 life ability", refused: "granted mana ability whose cost is not {T}"},
}

func TestGrantedManaRestrictCost(t *testing.T) { checkForms(t, grantedManaRestrictCostForms) }
