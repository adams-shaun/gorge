package kgen

import "testing"

// gainsabilitiesofForms: GainsAbilitiesOf$ on the battlefield for an
// AffectedDefined$ Self recipient lowers to st:GainsOf. An untyped source
// (Territory Forge) lowers: the kernel's offer walk matches any zone card and
// prices both mana and non-mana abilities, and StallGainsOf refuses the
// matchup when a lender actually carries an unmodelled one. An artifact or
// land source is refused by name (Mirran Safehouse's graveyard lands diverge
// from gorge on which source taps).
var gainsabilitiesofForms = []formCase{
	{card: "Experiment Kraj", form: "Creature.Other+counters_GE1_P1P1 on the battlefield", lir: "gainsof="},
	{card: "Robaran Mercenaries", form: "Creature.Legendary+YouCtrl on the battlefield", lir: "gainsof="},
	{card: "Necrotic Ooze", form: "Creature in the graveyard", lir: "gainsof="},
	{card: "Thranduil, the Elvenking", form: "Elf.YouOwn in the graveyard", lir: "gainsof="},
	{card: "Dark Impostor", form: "Creature.ExiledWithSource in exile", lir: "gainsof="},
	{card: "Patchwork Crawler", form: "Creature.ExiledWithSource in exile", lir: "gainsof="},
	{card: "Hazel's Brewmaster", form: "Affected$ Food.YouCtrl recipients", lir: "gainsof="},
	{card: "Marvin, Murderous Mimic", form: "Creature.YouCtrl+!sharesNameWith Self", lir: "gainsof="},
	{card: "Territory Forge", form: "untyped Card source in exile", lir: "gainsof="},
	{card: "Trazyn the Infinite", form: "artifact sources", refused: "artifact or land sources"},
	{card: "Mirran Safehouse", form: "land sources in the graveyard", refused: "artifact or land sources"},
}

func TestGainsAbilitiesOfForms(t *testing.T) { checkForms(t, gainsabilitiesofForms) }
