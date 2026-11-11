package kgen

import "testing"

// playercountCmdrRefusalForms pins that kgen keeps refusing the four
// Commander cards whose count forms engine/effects now models but the kernel
// does not lower: the refusal is the allowed alternative to lowering (no
// kernel, kop, oracle or kfuzz change). The test passes before and after the
// gorge-side feature -- its value is the pin.
var playercountCmdrRefusalForms = []formCase{
	{
		card:    "Windfall",
		form:    "PlayerCountPlayers$HighestValidGraveyard,Library,Exile (multi-zone remembered count)",
		refused: "PlayerCountPlayers$HighestValidGraveyard,Library,Exile",
	},
	{
		card:    "Jace's Archivist",
		form:    "PlayerCountPlayers$HighestValidGraveyard,Library,Exile (multi-zone remembered count)",
		refused: "PlayerCountPlayers$HighestValidGraveyard,Library,Exile",
	},
	{
		card:    "Iron Mastiff",
		form:    "RollDice Amount$ NbAttackedPlayers (one die per attacked player)",
		refused: `RollDice Amount$ "NbAttackedPlayers"`,
	},
	{
		card:    "Battle Angels of Tyr",
		form:    "TriggeredTarget$CardsInHand and the non-triggered-target count group",
		refused: "TriggeredTarget$CardsInHand",
	},
}

func TestPlayerCountCmdrRefusals(t *testing.T) {
	checkForms(t, playercountCmdrRefusalForms)
}
