package kgen

import "testing"

// huntStaticsForms: the three static modes gorge's rules engine reads
// (CantPayLife, CanAttackIfHaste, AttackRequirement) are not lowered by kgen:
// each card is refused by name with the static-mode reason, so dirac never
// plays a card whose static it cannot model. War's Toll also refuses the
// TapsForMana trigger (already gorge-supported; not part of this change).
var huntStaticsForms = []formCase{
	{card: "Angel of Jubilation", form: "S:Mode$ CantPayLife", refused: "static mode CantPayLife"},
	{card: "Instill Energy", form: "S:Mode$ CanAttackIfHaste", refused: "static mode CanAttackIfHaste"},
	{card: "War's Toll", form: "S:Mode$ AttackRequirement", refused: "static mode AttackRequirement"},
}

// TestHuntStaticsRefusedByKgen pins kgen's refusal of the three statics by card name.
func TestHuntStaticsRefusedByKgen(t *testing.T) { checkForms(t, huntStaticsForms) }
