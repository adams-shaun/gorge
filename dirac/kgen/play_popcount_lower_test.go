package kgen

import "testing"

// TestPlayPopCountShapes: Plargg and Nassari's Defined$ Remembered.!ChosenCard
// with a literal Amount$ 2 lowers whole (lower_playpopcount.go). The counted
// Amount$ shapes and Defined$ Targeted keep their refusals by name: Chandra
// Ablaze's ChandraX, Talent of the Telepath's X, Harness the Storm.
func TestPlayPopCountShapes(t *testing.T) {
	if c := lowerNamedCard(t, "Plargg and Nassari"); len(c.Refusals) != 0 {
		t.Errorf("Plargg and Nassari refusals = %v, want none", c.Refusals)
	}
	for name, want := range map[string]string{
		"Chandra Ablaze":         `Play Amount "ChandraX"`,
		"Talent of the Telepath": `Play Amount "X"`,
		"Harness the Storm":      `Play Defined$ "Targeted"`,
	} {
		if c := lowerNamedCard(t, name); !hasRefusal(c.Refusals, want) {
			t.Errorf("%s refusals = %v, want %q", name, c.Refusals, want)
		}
	}
}
