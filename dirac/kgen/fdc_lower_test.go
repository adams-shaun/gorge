package kgen

import "testing"

// fdcLowered are Foundations Commander (FDC) precon cards whose last refusal
// a root cause of the fdc lane retired: Haven of the Spirit Dragon's
// "Planeswalker.Ugin" target (a planeswalker subtype, CR 205.3j), Fateful
// Absence's Investigate Defined$ TargetedController, Ram Through's trampling
// excess, Endless Atlas' Count$MostCardName, Tangleweave Armor's every-zone
// commander mana value, Padeem's greatestCMC_Artifact.
var fdcLowered = []string{"Haven of the Spirit Dragon", "Fateful Absence", "Ram Through", "Endless Atlas", "Tangleweave Armor", "Padeem, Consul of Innovation"}

// TestFDCLoweredCards lowers them in one corpus pool (its type vocabulary
// read, as the pool tables' build reads it), each with no refusal.
func TestFDCLoweredCards(t *testing.T) {
	p, err := loadCardNames(fdcLowered, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatalf("build: %v", err)
	}
	seen := 0
	for _, c := range lw.cards {
		for _, n := range fdcLowered {
			if c.Name != n {
				continue
			}
			seen++
			if len(c.Refusals) != 0 {
				t.Errorf("%s refusals = %v, want none", n, c.Refusals)
			}
		}
	}
	if seen != len(fdcLowered) {
		t.Fatalf("%d of %d cards lowered", seen, len(fdcLowered))
	}
}
