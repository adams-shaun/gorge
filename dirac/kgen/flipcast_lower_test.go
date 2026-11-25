package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// transformersCards carry K:More Than Meets the Eye (CR 702.162a) on the
// front and K:Living metal (CR 702.161a) on the back, and are blocked by
// nothing else: each lowers whole, the front recording the converted cast's
// cost (lower_flipcast.go).
var transformersCards = []string{"Goldbug, Humanity's Ally", "Starscream, Power Hungry", "Ultra Magnus, Tactician"}

// disturbCarriers print K:Disturb (CR 702.146a) on a front whose back
// exiles itself instead of reaching a graveyard.
var disturbCarriers = []string{"Beloved Beggar", "Kindly Ancestor"}

func TestTransformersKwLower(t *testing.T) {
	for _, name := range transformersCards {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
		if c.FlipCast != kop.FlipConverted || c.FlipCost == nil {
			t.Errorf("%s: FlipCast = %d (cost %v), want the converted cast", name, c.FlipCast, c.FlipCost)
		}
	}
}

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
