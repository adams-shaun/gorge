package kgen

import "testing"

// chooseSourceForms pins the ChooseSource chains that lower (the next-damage
// prevention from a source of your choice, recipient You, any or the
// Effect's target, the Effect's self-exile body or a bodyless Prevent$ True
// for the turn) and the ones that stay refused by name.
var chooseSourceForms = []formCase{
	{card: "Circle of Protection: Red", form: "red-source pool, the shield re-checks the colour", lir: "ChooseSource"},
	{card: "Circle of Protection: Artifacts", form: "artifact pool", lir: "PreventNext"},
	{card: "Rune of Protection: Lands", form: "land pool", lir: "ChooseSource"},
	{card: "Greater Realm of Preservation", form: "black or red pool", lir: "ChooseSource"},
	{card: "Invulnerability", form: "a spell, every source", lir: "ChooseSource"},
	{card: "Righteous Aura", form: "RememberObjects$ Remembered beside the Effect", lir: "PreventNext"},
	{card: "Haazda Shield Mate", form: "creature with the activated shield", lir: "ChooseSource"},
	{card: "Rhystic Circle", form: "UnlessCost$ any player may pay before the pick", refused: "ChooseSource UnlessCost$"},
	{card: "Auriok Replica", form: "prevent all damage to you this turn (Prevent$ True)", lir: "C=1158"},
	{card: "Pay No Heed", form: "prevent all damage this turn, any recipient", lir: "C=1670"},
	{card: "Burrenton Forge-Tender", form: "a red source, all damage this turn, any recipient", lir: "PreventNext A=8 C=1670"},
	{card: "Pilgrim of Justice", form: "the next damage to any recipient (Card.ChosenCard+RedSource)", lir: "PreventNext A=8 C=646"},
	{card: "Martyr's Cause", form: "the next damage to the Effect's target (RememberObjects$ Targeted)", lir: "PreventNext obj=Target0 A=0 C=390"},
	{card: "Samite Blessing", form: "the next damage to the Effect's target (ImprintCards$ Targeted)", lir: "PreventNext obj=Target0 A=0 C=390"},
	{card: "Kithkin Armor", form: "ImprintCards$ AttachedBy Sacrificed", refused: "replacement shape"},
	{card: "Beacon of Destiny", form: "damage dealt to the Effect's source instead", refused: "replacement param DamageTarget"},
	{card: "Deflecting Palm", form: "damage dealt to the source's controller instead", refused: "replacement shape"},
	{card: "Cho-Arrim Alchemist", form: "gain the prevented life, then the self-exile", lir: "PreventNext A=0 C=142"},
	{card: "Shadowbane", form: "a gain body behind a ConditionCheckSVar$", refused: "replacement shape"},
}

// TestChooseSourceShapeForms pins the lowered and refused ChooseSource shapes.
func TestChooseSourceShapeForms(t *testing.T) { checkForms(t, chooseSourceForms) }
