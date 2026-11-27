package kgen

import "testing"

// changeTypeValidTgtsForms: "search target player's library" with ValidTgts$ a
// player filter and no DefinedPlayer$ (lower_zone.go targetLibrarySearch): the
// Search names the target slot's library and the resolving controller chooses
// (SrChooserYou), gorge's searchTargetedOwner shape. The targeted player's own
// library lowers too: into the hand or onto the battlefield with Chooser$
// Targeted and no DefinedPlayer$ (Fertilid, Fertilid's Favor, the owners read
// off the targets), and the targeted opponent's pick among the remembered
// cards of your own library (Intuition, Gifts Ungiven: SrChooserOpp, then the
// sweep of the rest that shuffles only when a swept card left the library).
var changeTypeValidTgtsForms = []formCase{
	{card: "Acquire", form: "SP$ ChangeZone Origin$ Library ValidTgts$ Opponent GainControl$", lir: "Search obj=Target0 A=1 C=1 CS=UnderYourControl|ChooserYou F={types=0x4}"},
	{card: "Bribery", form: "SP$ ChangeZone Origin$ Library ValidTgts$ Opponent ChangeType$ Creature", lir: "Search obj=Target0 A=1 C=1 CS=UnderYourControl|ChooserYou F={types=0x2}"},
	{card: "Jester's Cap", form: "AB$ ChangeZone ValidTgts$ Player Chooser$ You to exile", lir: "Search obj=Target0 A=3 C=5 CS=Mandatory|ChooserYou F={types=0x0}"},
	{card: "Fertilid", form: "Chooser$ Targeted, no DefinedPlayer$, onto the battlefield", lir: "Search obj=Target0 A=1 C=1 CS=Tapped F={types=0x1 Basic}"},
	{card: "Fertilid's Favor", form: "Chooser$ Targeted, no DefinedPlayer$, onto the battlefield", lir: "Search obj=Target0 A=1 C=1 CS=Tapped F={types=0x1 Basic}"},
	{card: "Intuition", form: "Chooser$ Targeted over the remembered cards", lir: "Search obj=You A=1 C=2 CS=Mandatory|NoShuffle|ChooserOpp F={types=0x0 Remembered}"},
	{card: "Gifts Ungiven", form: "Chooser$ Targeted over the remembered cards", lir: "Search obj=You A=2 C=3 CS=Mandatory|NoShuffle|ChooserOpp F={types=0x0 Remembered}"},
}

func TestChangeTypeValidTgtsForms(t *testing.T) { checkForms(t, changeTypeValidTgtsForms) }
