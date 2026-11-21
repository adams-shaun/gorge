package kgen

import "testing"

// kextraBlockForms: the creature's own CanBlockAmount$ 1 is lowered; every
// other extra-block static stays refused by its own key.
var kextraBlockForms = []formCase{
	{card: "Ghastbark Twins", form: "the self CanBlockAmount$ 1 static is lowered to CardDef.ExtraBlocks", lir: "extrablocks=1"},
	{card: "Wall of Glare", form: "the self CanBlockAny$ static is lowered to CardDef.ExtraBlocks = kop.ExtraBlocksAny", lir: "extrablocks=255"},
	{card: "Entangler", form: "the enchanted creature's CanBlockAny$ grant is lowered to a StatExtraBlock of amount any", lir: "blockamount=255"},
	{card: "Give No Ground", form: "a Pump's CanBlockAny$ stays refused (gorge's Pump does not read it)", refused: "unread params [CanBlockAny]"},
	{card: "Vanguard's Shield", form: "a CanBlockAmount$ grant through Affected$ stays refused by name", refused: `CanBlockAmount$ "1": a blocker of several attackers`},
}

func TestExtraBlockRefusedByName(t *testing.T) { checkForms(t, kextraBlockForms) }
