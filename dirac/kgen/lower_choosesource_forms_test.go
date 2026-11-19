package kgen

import "testing"

// chooseSourceForms pins the ChooseSource chains that lower (the next-damage
// prevention from a source of your choice, recipient You, the Effect's
// self-exile body) and the ones that stay refused by name.
var chooseSourceForms = []formCase{
	{card: "Circle of Protection: Red", form: "red-source pool, the shield re-checks the colour", lir: "ChooseSource"},
	{card: "Circle of Protection: Artifacts", form: "artifact pool", lir: "PreventNext"},
	{card: "Rune of Protection: Lands", form: "land pool", lir: "ChooseSource"},
	{card: "Greater Realm of Preservation", form: "black or red pool", lir: "ChooseSource"},
	{card: "Invulnerability", form: "a spell, every source", lir: "ChooseSource"},
	{card: "Righteous Aura", form: "RememberObjects$ Remembered beside the Effect", lir: "PreventNext"},
	{card: "Haazda Shield Mate", form: "creature with the activated shield", lir: "ChooseSource"},
	{card: "Rhystic Circle", form: "UnlessCost$ any player may pay before the pick", refused: "ChooseSource UnlessCost$"},
	{card: "Auriok Replica", form: "prevent all damage this turn (Prevent$ True)", refused: "replacement param Prevent"},
	{card: "Beacon of Destiny", form: "damage dealt to the Effect's source instead", refused: "replacement param DamageTarget"},
	{card: "Deflecting Palm", form: "damage dealt to the source's controller instead", refused: "replacement shape"},
	{card: "Cho-Arrim Alchemist", form: "gain life instead", refused: "replacement shape"},
}

// TestChooseSourceShapeForms pins the lowered and refused ChooseSource shapes.
func TestChooseSourceShapeForms(t *testing.T) { checkForms(t, chooseSourceForms) }
