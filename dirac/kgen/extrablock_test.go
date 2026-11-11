package kgen

import "testing"

// kextraBlockForms: a Continuous static granting extra blocks stays refused,
// now by its own key rather than as an anonymous unread param.
var kextraBlockForms = []formCase{
	{card: "Ghastbark Twins", form: "CanBlockAmount$ stays refused by name", refused: `CanBlockAmount$ "1": a blocker of several attackers`},
	{card: "Wall of Glare", form: "CanBlockAny$ stays refused by name", refused: `CanBlockAny$ "True": a blocker of several attackers`},
}

func TestExtraBlockRefusedByName(t *testing.T) { checkForms(t, kextraBlockForms) }
