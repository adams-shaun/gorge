package kgen

import "testing"

// handOfEmrakulForms: Annihilator 1 (CR 702.86a) lowers from gorge's
// expansion, its marker the Sacrifice count; the Eldrazi Spawn sacrifice
// alternative cost lowers on a creature spell.
var handOfEmrakulForms = []formCase{
	{card: "Hand of Emrakul", form: "Annihilator$ N is the defending player's Sacrifice count", lir: "Sacrifice"},
}

func TestHandOfEmrakulForms(t *testing.T) { checkForms(t, handOfEmrakulForms) }
