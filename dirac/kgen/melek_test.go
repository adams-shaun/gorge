package kgen

import "testing"

// kmelekForms: (OnlyFirstSpell$ with
// ValidSpell$): Melek's static gate is the ValidSpell$ filter and the
// first-spell ledger reads the stripped card spec; Vine Gecko's
// Spell.Kicked and Head of the Class's ValidCard$ + ValidSpell$ pair stay
// refused.
var kmelekForms = []formCase{
	{card: "Melek, Reforged Researcher", form: "ReduceCost OnlyFirstSpell$ ValidSpell$ lowers", lir: "static ReduceCost F={types=0x20 | types=0x40}"},
	{card: "Melek, Reforged Researcher", form: "the first-spell ledger condition", lir: "ThisTurnCastSpec({types=0x20 | types=0x40})"},
	{card: "Vine Gecko", form: "ValidSpell$ Spell.Kicked stays refused", refused: `unknown property "Kicked"`},
	{card: "Head of the Class", form: "ValidCard$ + ValidSpell$ stays refused", refused: "static unread params"},
}

func TestReducegatesValidSpell(t *testing.T) { checkForms(t, kmelekForms) }
