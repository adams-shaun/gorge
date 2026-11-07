package kgen

import (
	"testing"
)

// TestPrintedDelveLowers: the printed K:Delve cards of Wretched Ranks lower
// to CardDef.Delve and build; before the printed arm both were refused with
// keyword "Delve". The kernel reads the printed field beside a granted
// StatStackKw Delve (dirac/kw_delve.go).
func TestPrintedDelveLowers(t *testing.T) {
	names := []string{"Gurmag Angler", "Murderous Cut"}
	lw := lowerNamedPool(t, names)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatalf("build: %v", err)
	}
	seen := 0
	for _, c := range lw.cards {
		if c.Name != "Gurmag Angler" && c.Name != "Murderous Cut" {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		if !c.Delve {
			t.Errorf("%s: CardDef.Delve not set", c.Name)
		}
	}
	if seen != len(names) {
		t.Fatalf("lowered %d of %d cards", seen, len(names))
	}
}
