package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestRestrictedManaTapXMutationHidden: without the restricted tap-another
// source (DebugNoRestrTapX hides Grand Architect beside the pod's other
// restricted units) the two-seat pod diverges: its plays spend the mana and
// need it.
func TestRestrictedManaTapXMutationHidden(t *testing.T) {
	if isolate(t) {
		return
	}
	p := newRestrictedManaPod(t, 2)
	dirac.DebugNoRestrTapX = true
	defer func() { dirac.DebugNoRestrTapX = false }()
	for _, seed := range restrictedManaTapXSeeds {
		if res, _ := p.play(t, seed, 40); res.Div != nil {
			t.Logf("mutation diverged at seed %d decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("no seed diverged without the restricted tap-another source: the pod never needs Grand Architect's mana")
}
