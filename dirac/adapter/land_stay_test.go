package adapter

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

func TestReturnFitsReadsTheFilters(t *testing.T) {
	land := &cards.Face{Types: []string{"Land", "Island"}}
	for _, tc := range []struct {
		filter string
		tapped bool
		want   bool
	}{
		{"Land.YouCtrl", false, true},
		{"Land", true, true},
		{"Land.nonLair", false, true},
		{"Land.nonIsland", false, false},
		{"Island.untapped+YouCtrl", false, true},
		{"Island.untapped+YouCtrl", true, false},
		{"Plains.untapped", false, false},
		{"Creature.Other", false, false},
	} {
		if got := returnFits(tc.filter, land, tc.tapped); got != tc.want {
			t.Errorf("%s tapped=%v: %v", tc.filter, tc.tapped, got)
		}
	}
}
