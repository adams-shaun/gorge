package kgen

import "testing"

// huntStaticsForms: a static mode gorge's rules engine reads that kgen still
// refuses (CantPayLife) refuses its card by name with the static-mode reason,
// so dirac never plays a card whose static it cannot model; a static that
// lowers is pinned by its IR.
//
// AttackRequirement lowers since War's Toll's whole-declaration closure
// (gorge's AttackRequirementClauses): ValidCard$ binds the members and
// ValidAttacker$ the creatures whose attacking turns it on. The row pins
// War's Toll's lowered static rather than a refusal.
//
// CanAttackIfHaste used to sit on this list; it lowers since the MBC combat-5
// class (Frenzied Saddlebrute's defender scope, Instill Energy's enchanted
// scope, Chaos Lord's Self+!ThisTurnEntered scope beside its upkeep
// GainControl, lockstepped in dirac/oracle's TestChaosLordLockstep). The row
// moved to pinning Chaos Lord's lowered static: the shapes the static arm
// still refuses (a param outside gorge's readable whitelist
// CanAttackIfHasteParamsReadable, an unlowerable ValidCard$/ValidTarget$) have
// no corpus carrier.
var huntStaticsForms = []formCase{
	{card: "Angel of Jubilation", form: "S:Mode$ CantPayLife", refused: "static mode CantPayLife"},
	{card: "Chaos Lord", form: "S:Mode$ CanAttackIfHaste", lir: "static CanAttackIfHaste F={types=0x0 Self cmp(201 0 2  )}"},
	{card: "War's Toll", form: "S:Mode$ AttackRequirement", lir: "static AttackRequirement F={types=0x2 ctrl=2} F2={types=0x2 ctrl=2}"},
}

// TestHuntStaticsRefusedByKgen pins the static mode kgen refuses by card name,
// and the lowered CanAttackIfHaste and AttackRequirement statics.
func TestHuntStaticsRefusedByKgen(t *testing.T) { checkForms(t, huntStaticsForms) }
