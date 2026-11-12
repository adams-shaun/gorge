package kgen

import "testing"

// playercountCmdrForms pins what kgen does with the four Commander cards whose
// count forms engine/effects models. The two zone-scoped extreme carriers now
// lower onto the kernel's ExPlExtreme (with the pump-duration class) and are
// played decision-by-decision by TestOneRingLockstep; the dice and
// triggered-target forms stay refused, the allowed alternative to lowering.
var playercountCmdrForms = []formCase{
	{
		card: "Windfall",
		form: "PlayerCountPlayers$HighestValidGraveyard,Library,Exile (multi-zone remembered count)",
		lir:  "A=PlExtreme(",
	},
	{
		card: "Jace's Archivist",
		form: "PlayerCountPlayers$HighestValidGraveyard,Library,Exile (multi-zone remembered count)",
		lir:  "A=PlExtreme(",
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
	checkForms(t, playercountCmdrForms)
}
