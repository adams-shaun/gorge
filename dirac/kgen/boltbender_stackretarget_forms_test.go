package kgen

import "testing"

// boltbenderForms pins Boltbender's turn-face-up redirect: ChangeTargets over
// Defined$ ValidStack SpellAbility with Optional$ True lowers to the mass
// stack retarget (ExtChangeTargetsMass), not the single-entry ExtChangeTargets.
var boltbenderForms = []formCase{
	{card: "Boltbender", form: "ChangeTargets Defined$ ValidStack SpellAbility, Optional$ True", lir: "ChangeTargetsMass"},
}

func TestBoltbenderLowering(t *testing.T) { checkForms(t, boltbenderForms) }
