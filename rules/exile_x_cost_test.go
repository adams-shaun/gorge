package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestParseCostExileFromGraveX(t *testing.T) {
	got := ParseCost("X T ExileFromGrave<X/Card>")
	if got.X != 1 || !got.Tap || got.Generic != 0 || len(got.Unknown) != 0 || len(got.Exile) != 1 {
		t.Fatalf("parsed cost = %+v", got)
	}
	part := got.Exile[0]
	if !part.Announced || part.N != 0 || part.Zone != state.ZGraveyard || part.Spec != "Card" {
		t.Fatalf("exile part = %+v", part)
	}
	fixed := ParseCost("ExileFromGrave<1/Card>")
	if len(fixed.Exile) != 1 || fixed.Exile[0].N != 1 || fixed.Exile[0].Announced || fixed.Exile[0].Zone != state.ZGraveyard {
		t.Fatalf("fixed cost changed: %+v", fixed)
	}
}
