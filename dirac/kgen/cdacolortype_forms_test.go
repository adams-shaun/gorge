package kgen

import "testing"

// cdacolortypeForms: a colour or type characteristic-defining static on the
// card itself (CR 604.3, 613.3) lowers into the printed record, which every
// zone reads (lower_cdacolortype.go). Mistform Ultimus lowers to the
// Changeling record, which the IR dump does not print, so it asserts no refusal.
var cdacolortypeForms = []formCase{
	{card: "Transguild Courier", form: "CDA SetColor$ All", lir: `colors="WUBRG"`},
	{card: "Ghostfire", form: "CDA SetColor$ Colorless", lir: `colors="" pt=0/0`},
	{card: "Stonework Packbeast", form: "CDA AddType$ subtype list", lir: "sub=[Beast Cleric Rogue Warrior Wizard]"},
	{card: "Mistform Ultimus", form: "CDA AddAllCreatureTypes$ True"},
	{card: "Faceless One", form: "CDA SetColor$ ChosenColor stays refused by name", refused: `CDA SetColor "ChosenColor"`},
}

func TestCDAColorTypeForms(t *testing.T) { checkForms(t, cdacolortypeForms) }
