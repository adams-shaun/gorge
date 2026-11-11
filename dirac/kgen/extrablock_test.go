package kgen

import "testing"

// kextraBlockForms: the creature's own CanBlockAmount$ 1 is lowered; every
// other extra-block static stays refused by its own key.
var kextraBlockForms = []formCase{
	{card: "Ghastbark Twins", form: "the self CanBlockAmount$ 1 static is lowered to CardDef.ExtraBlocks", lir: "extrablocks=1"},
	{card: "Wall of Glare", form: "CanBlockAny$ stays refused by name", refused: `CanBlockAny$ "True": a blocker of several attackers`},
	{card: "Vanguard's Shield", form: "a CanBlockAmount$ grant through Affected$ stays refused by name", refused: `CanBlockAmount$ "1": a blocker of several attackers`},
}

func TestExtraBlockRefusedByName(t *testing.T) { checkForms(t, kextraBlockForms) }
