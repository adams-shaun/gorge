package kgen

import "testing"

// playerPumpGrantForms pins the answers to a Pump keyword grant to the
// resolving controller. With Duration$ UntilYourNextTurn (pumpPlayerGrant) a
// protection-from-everything grant lowers onto a next-turn emblem of the
// controller, which the kernel's plProtAll reads (static_plprot.go), and a
// hexproof grant refuses: the kernel's player hexproof readers
// (playerHexproof, youHexproof) walk the permanents a player controls and
// never an emblem, so a lowered grant would leave the player a legal target
// for an opponent where gorge refuses one. Until end of turn (pumpPlayerEOT)
// hexproof is the TurnReplPlayerHex record those readers do see, and shroud,
// which no kernel player read carries, refuses: neither may lower to a
// keyword grant, which lands on no permanent and so does nothing.
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
	{
		card: "The Needs of the Many",
		form: "Defined$ You KW$ Hexproof",
		lir:  "TurnRepl C=5",
	},
	{
		card: "Earthshape",
		form: "Defined$ You KW$ Hexproof",
		lir:  "TurnRepl C=5",
	},
	{
		card:    "Gilded Light",
		form:    "Defined$ You KW$ Shroud",
		refused: `player-scoped Pump grant keyword "Shroud" until end of turn`,
	},
}

func TestPlayerPumpGrantForms(t *testing.T) {
	checkForms(t, playerPumpGrantForms)
}
