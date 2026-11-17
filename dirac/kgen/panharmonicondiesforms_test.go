package kgen

import "testing"

// panharmoniconDiesForms: the dies-cause Panharmonicon (ValidCause$ Creature |
// Origin$ Battlefield | Destination$ Graveyard) lowers to a static whose A is
// kop.PanhModeDies and whose Filter2 is the ValidCause$ filter.
var panharmoniconDiesForms = []formCase{
	{card: "Teysa Karlov", form: "Panharmonicon Destination$ Graveyard (dies cause)", lir: "static Panharmonicon F={types=0x9f ctrl=1} F2={types=0x2} cond=true"},
	{card: "Drivnod, Carnage Dominus", form: "Panharmonicon Destination$ Graveyard (dies cause)", lir: "static Panharmonicon F={types=0x9f ctrl=1} F2={types=0x2} cond=true"},
	{card: "Starfield Vocalist", form: "Panharmonicon Destination$ Battlefield (ETB cause) still lowers", lir: "Panharmonicon"},
}

func TestPanharmoniconDiesForms(t *testing.T) { checkForms(t, panharmoniconDiesForms) }
