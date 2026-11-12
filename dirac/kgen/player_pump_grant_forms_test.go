package kgen

import "testing"

// playerPumpGrantForms pins pumpPlayerGrant's two keyword answers. A
// protection-from-everything grant lowers onto a next-turn emblem of the
// resolving controller, which the kernel's plProtAll reads
// (static_plprot.go). A hexproof grant refuses: the kernel's player hexproof
// readers (playerHexproof, youHexproof) walk the permanents a player
// controls and never an emblem, so a lowered grant would leave the player a
// legal target for an opponent where gorge refuses one.
var playerPumpGrantForms = []formCase{
	{
		card: "The Stasis Coffin",
		form: "Defined$ You KW$ Protection from everything Duration$ UntilYourNextTurn",
		lir:  "CreateEmblem obj=You",
	},
	{
		card:    "Blossoming Calm",
		form:    "Defined$ You KW$ Hexproof Duration$ UntilYourNextTurn",
		refused: `player-scoped Pump grant keyword "Hexproof": the kernel reads a player's hexproof off the permanents they control`,
	},
}

func TestPlayerPumpGrantForms(t *testing.T) {
	checkForms(t, playerPumpGrantForms)
}
