package kgen

import "testing"

// changeTypeValidTgtsForms: "search target player's library" with ValidTgts$ a
// player filter and no DefinedPlayer$ (lower_zone.go targetLibrarySearch): the
// Search names the target slot's library and the resolving controller chooses
// (SrChooserYou), gorge's searchTargetedOwner shape; a Chooser$ Targeted pick
// keeps refusing.
var changeTypeValidTgtsForms = []formCase{
	{card: "Acquire", form: "SP$ ChangeZone Origin$ Library ValidTgts$ Opponent GainControl$", lir: "Search obj=Target0 A=1 C=1 CS=UnderYourControl|ChooserYou F={types=0x4}"},
	{card: "Bribery", form: "SP$ ChangeZone Origin$ Library ValidTgts$ Opponent ChangeType$ Creature", lir: "Search obj=Target0 A=1 C=1 CS=UnderYourControl|ChooserYou F={types=0x2}"},
	{card: "Jester's Cap", form: "AB$ ChangeZone ValidTgts$ Player Chooser$ You to exile", lir: "Search obj=Target0 A=3 C=5 CS=Mandatory|ChooserYou F={types=0x0}"},
	{card: "Intuition", form: "Chooser$ Targeted over the remembered cards", refused: "ChangeType$ with ValidTgts$"},
}

func TestChangeTypeValidTgtsForms(t *testing.T) { checkForms(t, changeTypeValidTgtsForms) }
