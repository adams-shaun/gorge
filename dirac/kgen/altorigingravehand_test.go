package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAltOriginGraveHand: Agency Outfitter's library, graveyard and hand
// search lowers with the Graveyard,Hand pair in the SrAltOrigin zone code,
// and any other alternative list (or order) is refused by name.
func TestAltOriginGraveHand(t *testing.T) {
	all, bs, _, err := buildPool(lowerNamedPool(t, []string{"Agency Outfitter"}))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for i, c := range all {
		if c.Name != "Agency Outfitter" {
			continue
		}
		seen++
		if rs := lowerRefusals(c); len(rs) != 0 {
			t.Fatalf("Agency Outfitter refused: %v", rs)
		}
		found := false
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if in.Op != kop.OpSearchEx || in.B.N&kop.SrAltOrigin == 0 {
					continue
				}
				found = true
				if got := int(in.B.N>>kop.SrAltZoneShift) & 7; got != kop.SrAltGraveHand {
					t.Errorf("alt zone code %d, want SrAltGraveHand", got)
				}
				if in.B.N&kop.SrDifferentNames == 0 {
					t.Errorf("the pair search lost DifferentNames$")
				}
			}
		}
		if !found {
			t.Fatal("Agency Outfitter lowered without an SrAltOrigin search")
		}
	}
	if seen != 1 {
		t.Fatalf("built %d Agency Outfitter cards, want 1", seen)
	}

	for _, c := range []struct{ name, alt string }{
		{"reversed pair", "Hand,Graveyard"},
		{"three zones", "Graveyard,Hand,Library"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := czLowered(t, "SP$ ChangeZone | Origin$ Library | Destination$ Battlefield | ChangeType$ Card.namedMagnifying Glass | OriginAlternative$ "+c.alt+" | Hidden$ True | ChangeNum$ 2")
			kdefWant(t, got, `OriginAlternative$ "`+c.alt+`"`)
		})
	}
}
