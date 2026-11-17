package kgen

import "testing"

// alwaysIsPresentForms: Mode$ Always (CR 603.8) over an IsPresent$ filter
// other than the counter-threshold shape, lowered to the same Present(...)
// condition a static Condition$ uses.
var alwaysIsPresentForms = []formCase{
	{card: "Barbarian Outcast", form: "Swamp.YouCtrl EQ0", lir: "Present("},
	{card: "Bog Serpent", form: "Swamp.YouCtrl EQ0", lir: "Present("},
	{card: "Endrek Sahr, Master Breeder", form: "Card.Thrull+YouCtrl GE7", lir: "Present("},
	{card: "Afiya Grove", form: "Card.Self+counters_EQ0_P1P1", lir: "Present("},
}

func TestAlwaysIsPresentForms(t *testing.T) { checkForms(t, alwaysIsPresentForms) }
