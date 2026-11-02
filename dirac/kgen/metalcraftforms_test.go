package kgen

import "testing"

// metalcraftForms: a bare Condition$ Metalcraft reads the controller's
// artifact count (gorge metalcraftHolds, CR 207.2c), and a ChangeZone into the
// library with Shuffle$ True and LibraryPosition$ -1 places the card on the
// bottom before the shuffle (gorge effChangeZone's object path).
var metalcraftForms = []formCase{
	{card: "Dispatch", form: "Condition$ Metalcraft on a sub-ability: three or more artifacts the controller controls", lir: "SkipIf if=Value(op1,Count({types=0x4 ctrl=1}),3) skip=1"},
	{card: "Unlucky Cabbage Merchant", form: "Shuffle$ True with LibraryPosition$ -1: the source goes to the bottom, then the library is shuffled", lir: "ToLibrary obj=Self CS=BottomShuffle|from=Battlefield"},
	{card: "Astral Cornucopia", form: "three {X} symbols in the mana cost lower; the card itself stays refused by name", lir: "cost={0XXX}"},
}

func TestMetalcraftForms(t *testing.T) { checkForms(t, metalcraftForms) }
