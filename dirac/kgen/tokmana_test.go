package kgen

import "testing"

// ktokmanaForms: the granted mana ability shapes (lower_grantedmana.go
// grantedManaShape): a plain "{T}: Add <colours> x N" grant lowers; every
// other shape is refused with its reason.
var ktokmanaForms = []formCase{
	{card: "Springleaf Parade", form: "Continuous AddAbility$ of a plain {T}: Add any colour grant to creature tokens", lir: "Mana Granted cost=T"},
	{card: "Cryptolith Rite", form: "a plain any-colour grant to creatures you control", lir: "Mana Granted cost=T"},
	{card: "Chromatic Lantern", form: "a plain any-colour grant to lands you control (beside their own ability)", lir: "Mana Granted cost=T"},
	{card: "Gemhide Sliver", form: "a plain grant to every Sliver (either player's)", lir: "Mana Granted cost=T"},
	{card: "Sachi, Daughter of Seshiro", form: "a plain fixed two-mana grant ({G}{G})", lir: "mana G x2"},
	{card: "Clement, the Worrywort", form: "a granted ability with RestrictValid$", refused: "granted mana ability with a spend restriction"},
	{card: "Battery Bearer", form: "a granted ability with RestrictValid$ (artifact spells)", refused: "granted mana ability with a spend restriction"},
	{card: "Goldspan Dragon", form: "a granted Treasure ability ({T}, Sacrifice)", refused: "granted mana ability whose cost is not {T}"},
	{card: "Lotus Ring", form: "a granted sacrifice-only ability", refused: "granted mana ability whose cost is not {T}"},
	{card: "Forgotten Monument", form: "a granted {T}, Pay 1 life ability", refused: "granted mana ability whose cost is not {T}"},
	{card: "Evendo, Waking Haven", form: "a granted {G}, {T} ability with a dynamic amount", refused: "granted mana ability with a mana fee"},
	{card: "The Eternity Elevator", form: "a granted ability whose amount is a counter count", refused: "granted mana ability with a dynamic amount"},
	{card: "Thranduil the Strategist", form: "a granted Combo whose colour order ({G} or {U}) is not W U B R G", refused: "granted mana ability with a colour choice out of WUBRG order"},
}

func TestTokmanaForms(t *testing.T) { checkForms(t, ktokmanaForms) }
