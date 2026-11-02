package dirac

import "testing"

// TestTrigPlMarksDistinct: every spell-entry trigPl mark is its own value
// above the trigger-player range. Airbend once reused Madness's 0x81, so
// buildCostGen priced a madness cast at airbendCost (a wrong mana payment,
// and a panic in the cast window) whenever the table had an Airbend program.
func TestTrigPlMarksDistinct(t *testing.T) {
	marks := map[string]uint8{
		"sneak": trigPlSneak, "madness": trigPlMadness, "mayhem": trigPlMayhem, "airbend": trigPlAirbend,
		"web-slinging": trigPlWebSling,
	}
	seen := map[uint8]string{}
	for name, v := range marks {
		if v < 0x80 {
			t.Errorf("%s mark %#x is in the trigger-player range", name, v)
		}
		if prev, dup := seen[v]; dup {
			t.Errorf("%s and %s share trigPl mark %#x", prev, name, v)
		}
		seen[v] = name
	}
}
