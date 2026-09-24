package rules

import "testing"

func TestParseRemoveAnyCounterXMin(t *testing.T) {
	for _, input := range []string{
		"RemoveAnyCounter<X1+/P1P1/Creature>",
		"SubCounter<X1+/DREAM/NICKNAME>",
	} {
		c := ParseCost(input)
		if len(c.Unknown) != 0 || c.Generic != 0 || len(c.SubCounter) != 1 || !c.SubCounter[0].Announced || c.XMin != 1 {
			t.Errorf("ParseCost(%q) = generic %d XMin %d parts %#v unknown %q", input, c.Generic, c.XMin, c.SubCounter, c.Unknown)
		}
	}
}
