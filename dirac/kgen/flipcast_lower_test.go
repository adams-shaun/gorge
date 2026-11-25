package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// disturbCarriers print K:Disturb (CR 702.146a) on a front whose back
// exiles itself instead of reaching a graveyard.
var disturbCarriers = []string{"Beloved Beggar", "Kindly Ancestor"}

func TestDisturbKwLower(t *testing.T) {
	for _, name := range disturbCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
		if c.FlipCast != kop.FlipDisturb || c.FlipCost == nil {
			t.Errorf("%s: FlipCast = %d (cost %v), want the disturb cast", name, c.FlipCast, c.FlipCost)
		}
	}
}
