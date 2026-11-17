package kgen

import "testing"

// damageDoneReplShapeForms: the largest refused R: Event$ DamageDone sub-shape
// (ReplaceWith$ DB$ RemoveCounter, 6 of 27; .ds4 census): the self-damage
// prevention gated on ValidTarget$ Card.Self+counters_GE1_P1P1 (kop
// ReplPrevSelfIfCounter) removing one +1/+1 counter, or as many as the damage
// (kop ReplPrevSelfRemoveDmg). A body with a sub-ability keeps refusing.
var damageDoneReplShapeForms = []formCase{
	{card: "Undergrowth Champion", form: "Card.Self+counters_GE1_P1P1, remove one +1/+1 counter", lir: "repl PreventDamageCounters"},
	{card: "Oathsworn Knight", form: "Card.Self+counters_GE1_P1P1, remove one +1/+1 counter", lir: "repl PreventDamageCounters"},
	{card: "Ugin's Conjurant", form: "Card.Self+counters_GE1_P1P1, remove ReplaceCount$DamageAmount counters", lir: "repl PreventDamageCounters"},
	{card: "Bloatfly Swarm", form: "remove-counters body with a Radiation sub-ability", refused: "repl-hard: DamageDone prevent-and-remove-counter body"},
}

func TestDamageDoneReplShapeForms(t *testing.T) { checkForms(t, damageDoneReplShapeForms) }
