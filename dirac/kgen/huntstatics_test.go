package kgen

import "testing"

// huntStaticsForms: the static modes gorge's rules engine reads that kgen
// still refuses (CantPayLife, AttackRequirement) refuse their cards by name
// with the static-mode reason, so dirac never plays a card whose static it
// cannot model. War's Toll also refuses the TapsForMana trigger (already
// gorge-supported; not part of this change).
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
	{card: "War's Toll", form: "S:Mode$ AttackRequirement", refused: "static mode AttackRequirement"},
}

// TestHuntStaticsRefusedByKgen pins the two static modes kgen refuses by card
// name, and Chaos Lord's CanAttackIfHaste lowering.
func TestHuntStaticsRefusedByKgen(t *testing.T) { checkForms(t, huntStaticsForms) }
