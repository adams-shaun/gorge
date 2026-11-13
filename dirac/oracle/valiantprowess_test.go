package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A spell targeting Emberheart Challenger queues its Valiant before its prowess:
// the targeting comes before the cast is complete, so the two sides' trigger order
// options name the same triggers.
func TestTargetTriggerQueuesBeforeCastTrigger(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{{Name: "Emberheart Challenger", Count: 12}, {Name: "Might of the Meek", Count: 14}, {Name: "Mountain", Count: 14}}
	lockstepRun(t, "valiantprowess", [2][]cardset.Entry{deck, deck}, 24, true, nil)
}
