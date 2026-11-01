package kgen

import "testing"

// UnlessSwitched lowers to its own op so the kernel can tell an unless choice
// from a trigger's own Cost$ at the start of a trigger.
var unlessSwitchedForms = []formCase{
	{card: "Unifying Theory", form: "trigger-start UnlessSwitched (unless modes, not trigger_cost)", lir: "0 UnlessSwitched obj=TriggeredPlayer A=2 B=0 CS=Mana skip=1"},
}

func TestUnlessSwitchedForms(t *testing.T) { checkForms(t, unlessSwitchedForms) }
