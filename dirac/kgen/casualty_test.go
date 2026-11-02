package kgen

import "testing"

// casualtyForms: printed K:Casualty:<N> (CR 702.153a) lowers to
// CardDef.Casualty; the variable form is refused.
var casualtyForms = []formCase{
	{card: "Make Disappear", form: "K:Casualty:1 (CardDef.Casualty)", lir: " casualty=1"},
	{card: "Cut of the Profits", form: "K:Casualty:3 (CardDef.Casualty)", lir: " casualty=3"},
	{card: "Ob Nixilis, the Adversary", form: "Casualty:X with its NonLegendary$ / SetLoyalty$ riders: kept refused", refused: "variable casualty (Casualty:X"},
}

func TestCasualtyForms(t *testing.T) { checkForms(t, casualtyForms) }
