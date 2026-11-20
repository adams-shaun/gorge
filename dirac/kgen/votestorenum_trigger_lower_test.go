package kgen

import (
	"strings"
	"testing"
)

// TestVoteStoreNumTriggeredHasNoPlacementMode pins that a triggered
// StoreVoteNum$ fixed-list Vote lowers to the bare vote: gorge poses no CR
// 603.3c placement mode for a triggered Vote (trigger_queue.go
// askTriggerModes), so the kernel must not ask one either.
func TestVoteStoreNumTriggeredHasNoPlacementMode(t *testing.T) {
	carriers := []string{"Lieutenants of the Guard", "Messenger Jays", "Orchard Elemental", "Emissary Green"}
	lir, ref := lowerCarriers(t, carriers)
	for _, n := range carriers {
		for _, r := range ref[n] {
			t.Errorf("%s: refused: %s", n, r)
		}
		sec := section(lir, n)
		if !strings.Contains(sec, "Vote C=") {
			t.Errorf("%s: no Vote op:\n%s", n, sec)
		}
		if strings.Contains(sec, "Charm") {
			t.Errorf("%s: a triggered vote carries a placement Charm:\n%s", n, sec)
		}
	}
}
