package kgen

import "testing"

// filterHeadPlayerForms: Player.EnchantedBy, the player the source Aura enchants
// (kop.WhoseEnchanted = 202), as a printed battlefield Phase trigger's
// ValidPlayer$ and a DamageDone trigger's ValidTarget$ (lower_trigger.go); the
// other Player.<property> heads keep refusing by name.
var filterHeadPlayerForms = []formCase{
	{card: "Curse of Stalked Prey", form: "DamageDone ValidTarget$ Player.EnchantedBy", lir: "trigger DamageDone F={types=0x2} F2={players=3} whose=202"},
	{card: "Cruel Reality", form: "Phase ValidPlayer$ Player.EnchantedBy", lir: "whose=202"},
	{card: "Curse of Obsession", form: "two Phase ValidPlayer$ Player.EnchantedBy lines", lir: "whose=202"},
	{card: "Cartographer's Hawk", form: "Player.withMoreLandsThanYou (no kernel fact)", refused: `filter "Player.withMoreLandsThanYou"`},
	{card: "Grievous Wound", form: "DamageDoneOnce Player.EnchantedBy (the once-batch path)", refused: `filter "Player.EnchantedBy"`},
}

func TestFilterHeadPlayerForms(t *testing.T) { checkForms(t, filterHeadPlayerForms) }
