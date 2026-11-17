package kgen

import "testing"

// gainsabilitiesofForms: GainsAbilitiesOf$ on the battlefield for an
// AffectedDefined$ Self recipient lowers to st:GainsOf; other zones and
// artifact/land sources are refused by name.
var gainsabilitiesofForms = []formCase{
	{card: "Experiment Kraj", form: "Creature.Other+counters_GE1_P1P1 on the battlefield", lir: "gainsof="},
	{card: "Robaran Mercenaries", form: "Creature.Legendary+YouCtrl on the battlefield", lir: "gainsof="},
	{card: "Necrotic Ooze", form: "Creature in the graveyard", lir: "gainsof="},
	{card: "Thranduil, the Elvenking", form: "Elf.YouOwn in the graveyard", lir: "gainsof="},
	{card: "Dark Impostor", form: "Creature.ExiledWithSource in exile", lir: "gainsof="},
	{card: "Patchwork Crawler", form: "Creature.ExiledWithSource in exile", lir: "gainsof="},
	{card: "Hazel's Brewmaster", form: "Affected$ Food.YouCtrl recipients", lir: "gainsof="},
	{card: "Marvin, Murderous Mimic", form: "Creature.YouCtrl+!sharesNameWith Self", lir: "gainsof="},
	{card: "Territory Forge", form: "untyped Card source", refused: "artifact or land sources"},
	{card: "Trazyn the Infinite", form: "artifact sources", refused: "artifact or land sources"},
}

func TestGainsAbilitiesOfForms(t *testing.T) { checkForms(t, gainsabilitiesofForms) }
