package kgen

import "testing"

// TestChooseNumberMidMinPastDefaultKeepsTheFloor: a mid-resolution
// ChooseNumber with a Min$ past the default 0..12 list and no Max$ keeps the
// stated floor in the instruction. gorge's chooseNumberAsk fails that shape
// closed (0, unasked), and the kernel's chooseNumberMidBounds takes the same
// fallback from the same floor; zeroing Min$ here would make the kernel ask
// 0..12 instead, a question gorge never poses.
func TestChooseNumberMidMinPastDefaultKeepsTheFloor(t *testing.T) {
	c := lowerScriptCard(t, "Probe High Floor", "Name:Probe High Floor\nManaCost:1 U\nTypes:Instant\n"+
		"A:SP$ ChooseNumber | Min$ 13\nOracle:x\n")
	if len(c.Refusals) != 0 {
		t.Fatalf("refusals %v, want none", c.Refusals)
	}
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op != "ChooseNumberMid" {
				continue
			}
			if in.C&0xff != 13 {
				t.Fatalf("ChooseNumberMid floor %d, want the stated 13", in.C&0xff)
			}
			if in.C&0x100 != 0 {
				t.Fatalf("ChooseNumberMid C %#x claims a Max$ the card never states", in.C)
			}
			if in.C&0x200 != 0 {
				t.Fatalf("ChooseNumberMid C %#x claims Random$ True", in.C)
			}
			return
		}
	}
	t.Fatal("no ChooseNumberMid")
}
