package kgen

import "testing"

// TestRestrictedAmountLowering: a restricted Produced$ Any ability of a
// literal amount above one beside a fixed first ability (Intrepid
// Stablemaster) lowers, with its amount in ManaN; an ability restricted to
// activations only (Sunken Citadel) lowers too, as a restricted pair
// (lower_nmana_notfixedset_test.go). (Hermitic Herbalist's Combo of any
// colours: restrictedcombo_test.go.)
func TestRestrictedAmountLowering(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Intrepid Stablemaster", "Sunken Citadel"})
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, b := range bs {
		switch b.def.Name {
		case "Intrepid Stablemaster":
			seen++
			if len(b.refusals) != 0 {
				t.Errorf("Intrepid Stablemaster refused: %v", b.refusals)
			}
			n := 0
			for _, ab := range b.def.Abilities {
				if ab.ManaRestrict != 0 && ab.ManaN == 2 {
					n++
				}
			}
			if n != 1 {
				t.Errorf("Intrepid Stablemaster: %d restricted abilities of two mana, want 1", n)
			}
		}
	}
	if seen != 1 {
		t.Fatalf("Intrepid Stablemaster built %d times", seen)
	}
	for _, c := range lw.cards {
		if c.Name == "Sunken Citadel" && len(c.Refusals) != 0 {
			t.Errorf("Sunken Citadel refused: %v", c.Refusals)
		}
	}
}
