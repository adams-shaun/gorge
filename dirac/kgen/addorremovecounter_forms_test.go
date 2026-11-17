package kgen

import "testing"

// addOrRemoveCounterForms: the AddOrRemoveCounter carriers lower to the new
// Ext op (Plague Boiler's named plague counter, Quarry Hauler's each-kind
// loop, Sigurd's lore counter). Portent Tracker's RemoveConditionSVar$ is
// outside the supported shapes and refuses by name.
var addOrRemoveCounterForms = []formCase{
	{card: "Plague Boiler", form: "Defined$ Self, CounterType$ PLAGUE", lir: "AddOrRemoveCounter obj=Self A=1 CS=PLAGUE"},
	{card: "Quarry Hauler", form: "EachExistingCounter$ True over a target permanent", lir: "AddOrRemoveCounter obj=Target0 A=1 C=-1"},
	{card: "Sigurd, Jarl of Ravensthorpe", form: "CounterType$ LORE on a target Saga you control", lir: "AddOrRemoveCounter obj=Target0 A=1 CS=LORE"},
	{card: "Portent Tracker", form: "RemoveConditionSVar$", refused: "RemoveConditionSVar"},
}

func TestAddOrRemoveCounterForms(t *testing.T) { checkForms(t, addOrRemoveCounterForms) }
