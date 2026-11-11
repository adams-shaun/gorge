package kgen

import "testing"

// Cabal Ritual's Amount$ X (Count$Threshold.5.3, CR 702.24a) and Rite of
// Flame's SubAbility DB$ Mana (Amount$ X over the named graveyard cards) lower
// beside Dark Ritual's constant amount.
var ritualForms = []formCase{
	{card: "Cabal Ritual", form: "Count$Threshold arms as the mana amount", lir: "A=Compare(op=1,n=7,CountGraveyard({types=0x0 owner=1}),Branches(5,3)) CS=B"},
	{card: "Rite of Flame", form: "constant R R then the named-graveyard count", lir: "AddMana obj=You A=2 CS=R"},
	{card: "Rite of Flame", form: "the named-graveyard count (each graveyard) as the sub-ability amount", lir: "A=CountGraveyard({types=0x0}) CS=R"},
}

func TestRitualForms(t *testing.T) { checkForms(t, ritualForms) }
