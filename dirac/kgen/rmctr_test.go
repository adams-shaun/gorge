package kgen

import "testing"

// krmctrForms: the RemoveCounter Choices$ election's
// refusals.
var krmctrForms = []formCase{
	{card: "Garnet, Princess of Alexandria", form: "ChoiceOptional$ any number of Sagas lowers to the any-number remove form", lir: "0 Support A=16 C=10 CS=LORE"},
	{card: "Mister Hyde, Monster Within", form: "CounterType$ Any beside Choices$ stays refused (a per-pick kind ask)", refused: "RemoveCounter Choices$ with CounterType$ \"Any\""},
}

func TestRmctrLowers(t *testing.T) { checkForms(t, krmctrForms) }
