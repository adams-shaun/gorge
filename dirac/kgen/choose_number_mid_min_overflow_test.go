package kgen

import (
	"strings"
	"testing"
)

// TestChooseNumberMidMinPastTheFloorByteRefused: the kernel keeps the literal
// Min$ floor in the low byte of C beside the has-Max (0x100) and Random
// (0x200) flags. A larger literal would alias into a flag and pose a
// different question, so lowering must refuse it by name rather than keep a
// bound it cannot represent. The 255 boundary still fits.
func TestChooseNumberMidMinPastTheFloorByteRefused(t *testing.T) {
	c := lowerScriptCard(t, "Probe Wide Floor", "Name:Probe Wide Floor\nManaCost:1 U\nTypes:Instant\n"+
		"A:SP$ ChooseNumber | Min$ 512\nOracle:x\n")
	if len(c.Refusals) == 0 {
		t.Fatal("Min$ 512 lowered, want the floor-byte refusal")
	}
	found := false
	for _, r := range c.Refusals {
		if strings.Contains(r.Reason, "255 floor") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Min$ 512 refusals %v, want the 255 floor refusal", c.Refusals)
	}
}

// TestChooseNumberMidMinAtTheFloorByteBoundary: 255 is the largest literal
// the low byte keeps apart from the flags, and it still lowers with no flag
// set.
func TestChooseNumberMidMinAtTheFloorByteBoundary(t *testing.T) {
	c := lowerScriptCard(t, "Probe Floor Byte", "Name:Probe Floor Byte\nManaCost:1 U\nTypes:Instant\n"+
		"A:SP$ ChooseNumber | Min$ 255\nOracle:x\n")
	if len(c.Refusals) != 0 {
		t.Fatalf("Min$ 255 refusals %v, want none", c.Refusals)
	}
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op != "ChooseNumberMid" {
				continue
			}
			if in.C&0xff != 255 || in.C&0x100 != 0 || in.C&0x200 != 0 {
				t.Fatalf("ChooseNumberMid C %#x, want floor 255 and no flags", in.C)
			}
			return
		}
	}
	t.Fatal("no ChooseNumberMid")
}
