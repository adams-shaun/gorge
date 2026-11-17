package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Units the reach walk cannot tell apart (same alternatives, different source
// abilities) are twins of the nearest earlier one; a unit whose fee or colour
// differs is not.
func TestCastWindowTwinsPairIdenticalUnits(t *testing.T) {
	t.Parallel()
	red := func(ma *cards.SA) windowManaAlt {
		return windowManaAlt{Ma: ma, Counts: [6]int32{0, 0, 0, 1, 0, 0}, Amt: 1}
	}
	paid := func(ma *cards.SA, fee int32) windowManaAlt {
		return windowManaAlt{Ma: ma, Counts: [6]int32{0, 0, 0, 1, 0, 0}, Amt: 1, CostGeneric: fee}
	}
	a, b := &cards.SA{}, &cards.SA{}
	units := []windowManaUnit{
		{ID: 1, Alts: []windowManaAlt{red(a)}},
		{ID: 2, Alts: []windowManaAlt{paid(a, 1)}},
		{ID: 3, Alts: []windowManaAlt{red(b)}},
		{ID: 4, Alts: []windowManaAlt{paid(b, 2)}},
		{ID: 5, Alts: []windowManaAlt{paid(b, 1)}},
	}
	got := castWindowTwins(units)
	want := []int{-1, -1, 0, -1, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("twin[%d] = %d, want %d (all %v)", i, got[i], want[i], got)
		}
	}
}
