package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLandwalkNames: only an Or-chain link that is exactly the bare subtype
// filter counts, whatever zone or Or link it carries.
func TestLandwalkNames(t *testing.T) {
	tb := &kop.Table{Filters: []kop.Filter{
		{},                                  // 0: none
		{Sub: 7, Zone: kop.ZoneBattlefield}, // 1: Island
		{Sub: 5, Zone: kop.ZoneBattlefield, Or: 1},            // 2: Swamp, then Island
		{Sub: 7, Zone: kop.ZoneBattlefield, Types: kop.TLand}, // 3: a narrower filter
		{Sub: 5, Zone: kop.ZoneBattlefield},                   // 4: Swamp
	}}
	for _, c := range []struct {
		f    kop.FilterID
		sub  uint8
		want bool
	}{{1, 7, true}, {2, 7, true}, {3, 7, false}, {4, 7, false}, {0, 7, false}, {1, 0, false}} {
		if got := landwalkNames(tb, c.f, c.sub); got != c.want {
			t.Errorf("landwalkNames(filter %d, sub %d) = %v, want %v", c.f, c.sub, got, c.want)
		}
	}
}
