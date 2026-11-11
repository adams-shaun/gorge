package kgen

import "testing"

// Dark Ritual's lone SP$ Mana (Produced$ B, Amount$ 3, AI hints beside) lowers
// as a resolving AddMana of three black into the caster's pool (CR 106.4,
// 605.1a: a spell is no mana ability). The precon league once recorded it as
// refused, from a kgen older than the spell-adds-mana lowering.
var darkRitualForms = []formCase{
	{card: "Dark Ritual", form: "SP$ Mana Amount$ 3 adds BBB as the spell resolves", lir: "AddMana obj=You A=3 CS=B"},
}

func TestDarkRitualForms(t *testing.T) { checkForms(t, darkRitualForms) }
