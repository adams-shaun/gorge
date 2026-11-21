package payexec

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// A Produced$ of several symbols (Jungle Basin's "C G") labels its wheel
// option "Add C G": the production is one {C} and one {G}, not unreadable.
func TestLabelProductionReadsSpacedSymbols(t *testing.T) {
	amt, any, combo, ok := labelProduction("Add C G")
	if !ok || any || combo != nil {
		t.Fatalf("labelProduction(Add C G) = ok %v any %v combo %v", ok, any, combo)
	}
	var want decision.ManaAmount
	want[symIndex("C")]++
	want[symIndex("G")]++
	if amt != want {
		t.Fatalf("amount %v, want %v", amt, want)
	}
	if _, _, _, ok := labelProduction("Add "); ok {
		t.Fatal("an empty tail names no production")
	}
}
