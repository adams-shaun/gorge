package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestArrangeNotSearched: azmcts has no arrange searched kind, so an
// arrange pick (scry/surveil order, a dig's rest) is bot-answered like a
// mulligan bottom; ordinary choose and optional picks stay searched.
func TestArrangeNotSearched(t *testing.T) {
	k := FullKinds()
	const limit = 8
	tests := []struct {
		name string
		sub  dirac.ChoiceKind
		n    uint16
		want uint8
	}{
		{"arrange", dirac.ChArrange, 2, kindNone},
		{"bottom", dirac.ChBottom, 2, kindNone},
		{"discard", dirac.ChDiscard, 2, kindChoose},
		{"optional", dirac.ChOptional, 2, kindOptional},
	}
	for _, tt := range tests {
		d := dirac.Decision{Kind: dirac.DkChoose, Sub: tt.sub, N: tt.n}
		if got := kindOf(&d, &k, limit); got != tt.want {
			t.Errorf("%s: kindOf = %d, want %d", tt.name, got, tt.want)
		}
	}
}
