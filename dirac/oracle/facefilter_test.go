package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A face cast the planner cannot price takes the filter land's scripted prefix on
// the autopay surface too: only plain casts are dropped there.
func TestFaceCastThroughFilterLandLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{
		{Name: "Ardenvale Tactician", Count: 14}, {Name: "Crystal Grotto", Count: 12}, {Name: "Island", Count: 14},
	}
	lockstepRun(t, "facefilter", [2][]cardset.Entry{deck, deck}, 24, true, nil)
}
