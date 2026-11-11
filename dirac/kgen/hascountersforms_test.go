package kgen

import "testing"

// hascountersformsForms: sweep 1 (Standard-weighted) lowerings, one carrier
// per class, checked like paramreadformsForms.
var hascountersformsForms = []formCase{
	{card: "Bulwark Ox", form: "HasCounters (PredHasCounters) in a PumpAll ValidCards$", lir: "cmp(15 0 0  )"},
	{card: "Delta Bloodflies", form: "HasCounters in a trigger IsPresent$", lir: "cmp(15 0 0  )"},
	{card: "Damning Verdict", form: "!HasCounters is PredNoCounters", lir: "cmp(18 0 0  )"},
	{card: "Priest of Urabrask", form: "triggered DB$ Mana Produced$ R Amount$ 3 (gorge effMana ManaAmountNum)", lir: "AddMana obj=You A=3"},
	{card: "Cathodion", form: "an artifact's triggered Amount$ mana is typed: refused", refused: "typed mana"},
	{card: "Cautious Survivor", form: "trigger PresentDefined$ Self IsPresent$ Card.tapped is the source alone (gorge presentClauseHolds)", lir: "cond=Present({types=0x0 Self Tapped},op1,1)"},
}

func TestHasCountersAndTriggeredManaForms(t *testing.T) { checkForms(t, hascountersformsForms) }
