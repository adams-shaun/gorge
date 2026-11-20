package kgen

import (
	"strings"
	"testing"
)

// TestTriggeredVoteStoreNumHasNoPlacementCharm pins that a StoreVoteNum$
// fixed-list Vote on a triggered ability lowers as a spell's does: one Vote op
// and its branches, no OpCharm block and no modes on the ability. Gorge poses
// no CR 603.3c placement mode for a triggered Vote (engine/rules
// askTriggerModes), so the kernel must not ask one either.
func TestTriggeredVoteStoreNumHasNoPlacementCharm(t *testing.T) {
	carriers := []string{"Messenger Jays", "Lieutenants of the Guard", "Emissary Green", "Orchard Elemental"}
	lir, _ := lowerCarriers(t, carriers)
	for _, n := range carriers {
		sec := section(lir, n)
		if strings.Count(sec, "Vote C=") != 1 {
			t.Errorf("%s: want one Vote op:\n%s", n, sec)
		}
		if strings.Contains(sec, "Charm") {
			t.Errorf("%s: a triggered Vote carries a placement Charm:\n%s", n, sec)
		}
		if !strings.Contains(sec, "modes=0") {
			t.Errorf("%s: a triggered Vote ability has modes:\n%s", n, sec)
		}
	}
}
