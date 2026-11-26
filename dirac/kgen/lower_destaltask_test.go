package kgen

import "testing"

// TestEmeriaShepherdLowering: the landfall trigger's non-MANDATORY
// DestAltSVar$ lowers to the controller's primary-or-alternate ask
// (OpAskDestAlt) behind the condition guard, instead of refusing "without
// MANDATORY (a destination choice)".
func TestEmeriaShepherdLowering(t *testing.T) {
	c := lowerNamedCard(t, "Emeria Shepherd")
	if len(c.Refusals) != 0 {
		t.Fatalf("Emeria Shepherd refusals = %v, want none", c.Refusals)
	}
	asks := 0
	for _, ab := range c.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "AskDestAlt" {
				asks++
			}
		}
	}
	if asks != 1 {
		t.Errorf("AskDestAlt count = %d, want 1", asks)
	}
}
