package kgen

import "testing"

// ktokmanaForms: the granted mana ability shapes (lower_grantedmana.go
// grantedManaShape): a "{T}: Add <colours> x N" grant, plain or restricted
// to spells (and abilities), or a "{T}, Pay N life" grant, lowers; every
// other shape is refused with its reason.
var ktokmanaForms = []formCase{
	{card: "Springleaf Parade", form: "Continuous AddAbility$ of a plain {T}: Add any colour grant to creature tokens", lir: "Mana Granted cost=T"},
	{card: "Cryptolith Rite", form: "a plain any-colour grant to creatures you control", lir: "Mana Granted cost=T"},
	{card: "Chromatic Lantern", form: "a plain any-colour grant to lands you control (beside their own ability)", lir: "Mana Granted cost=T"},
	{card: "Gemhide Sliver", form: "a plain grant to every Sliver (either player's)", lir: "Mana Granted cost=T"},
	{card: "Sachi, Daughter of Seshiro", form: "a plain fixed two-mana grant ({G}{G})", lir: "mana G x2"},
	{card: "Clement, the Worrywort", form: "a granted Combo restricted to creature spells", lir: "Mana Granted cost=T"},
	{card: "Battery Bearer", form: "a granted ability restricted to artifact spells", lir: "Mana Granted cost=T"},
	{card: "Ambulatory Reactor", form: "a granted ability restricted to artifact spells and abilities of artifact sources", lir: "Mana Granted cost=T"},
	{card: "Goldspan Dragon", form: "a granted Treasure ability ({T}, Sacrifice)", refused: "granted mana ability whose cost is not {T}"},
	{card: "Lotus Ring", form: "a granted sacrifice-only ability", refused: "granted sacrifice-only mana ability: the kernel's manual mana window cannot activate it"},
	{card: "Forgotten Monument", form: "a granted {T}, Pay 1 life ability", lir: "Mana Granted cost=T Life1"},
	{card: "The Eternity Elevator", form: "a granted ability whose amount is a counter count", refused: "granted mana ability with a dynamic amount"},
	{card: "Thranduil the Strategist", form: "a granted Combo whose colour order ({G} or {U}) is not W U B R G", lir: "Mana Granted cost=T"},
}

func TestTokmanaForms(t *testing.T) { checkForms(t, ktokmanaForms) }
