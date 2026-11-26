package kgen

import "testing"

// TestPlayPopCountShapes: Plargg and Nassari's Defined$ Remembered.!ChosenCard
// with a literal Amount$ 2 lowers whole (lower_playpopcount.go). The counted
// Amount$ shapes and keep their refusals by name: Chandra Ablaze's ChandraX, Talent of the
// Telepath's X; Harness the Storm's Defined$ Targeted lowers.
func TestPlayPopCountShapes(t *testing.T) {
	for _, name := range []string{"Plargg and Nassari", "Harness the Storm"} {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
	for name, want := range map[string]string{
		"Chandra Ablaze":         `Play Amount "ChandraX"`,
		"Talent of the Telepath": `Play Amount "X"`,
	} {
		if c := lowerNamedCard(t, name); !hasRefusal(c.Refusals, want) {
			t.Errorf("%s refusals = %v, want %q", name, c.Refusals, want)
		}
	}
}
